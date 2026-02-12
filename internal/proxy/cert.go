package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CertManager handles CA certificate generation and per-host certificate minting.
type CertManager struct {
	caCert    *x509.Certificate
	caKey     *ecdsa.PrivateKey
	caTLS     tls.Certificate
	certCache sync.Map // map[string]*tls.Certificate
	certDir   string
}

// NewCertManager creates or loads the CA certificate from the config directory.
func NewCertManager(configDir string) (*CertManager, error) {
	cm := &CertManager{certDir: configDir}

	caPath := filepath.Join(configDir, "ca.pem")
	keyPath := filepath.Join(configDir, "ca-key.pem")

	if fileExists(caPath) && fileExists(keyPath) {
		if err := cm.loadCA(caPath, keyPath); err != nil {
			return nil, fmt.Errorf("load CA: %w", err)
		}
	} else {
		if err := cm.generateCA(caPath, keyPath); err != nil {
			return nil, fmt.Errorf("generate CA: %w", err)
		}
	}

	return cm, nil
}

// CACertPath returns the path to the CA certificate file.
func (cm *CertManager) CACertPath() string {
	return filepath.Join(cm.certDir, "ca.pem")
}

// GetCertificate returns a TLS certificate for the given hostname, generating one if needed.
func (cm *CertManager) GetCertificate(host string) (*tls.Certificate, error) {
	// Strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if cached, ok := cm.certCache.Load(host); ok {
		return cached.(*tls.Certificate), nil
	}

	cert, err := cm.mintCert(host)
	if err != nil {
		return nil, err
	}

	cm.certCache.Store(host, cert)
	return cert, nil
}

func (cm *CertManager) generateCA(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"GraphScope"},
			CommonName:   "GraphScope CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create CA cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse CA cert: %w", err)
	}

	// Save cert
	certFile, err := os.Create(certPath)
	if err != nil {
		return err
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certFile.Close()
		return fmt.Errorf("write CA cert: %w", err)
	}
	if err := certFile.Close(); err != nil {
		return fmt.Errorf("close CA cert file: %w", err)
	}

	// Save key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		keyFile.Close()
		return fmt.Errorf("write CA key: %w", err)
	}
	if err := keyFile.Close(); err != nil {
		return fmt.Errorf("close CA key file: %w", err)
	}

	cm.caCert = cert
	cm.caKey = key
	cm.caTLS = tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	return nil
}

func (cm *CertManager) loadCA(certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}

	// Parse certificate
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("no PEM block in CA cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	// Parse key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("no PEM block in CA key")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	cm.caCert = cert
	cm.caKey = key
	cm.caTLS = tls.Certificate{
		Certificate: [][]byte{block.Bytes},
		PrivateKey:  key,
	}

	return nil
}

// mintCert generates a TLS certificate for a specific host, signed by our CA.
func (cm *CertManager) mintCert(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	// Set SANs
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, cm.caCert, &key.PublicKey, cm.caKey)
	if err != nil {
		return nil, err
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER, cm.caTLS.Certificate[0]},
		PrivateKey:  key,
	}

	return tlsCert, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
