package tlsinterceptor

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

type TLSInterceptor struct {
	caCert *x509.Certificate
	caKey  crypto.PrivateKey
}

func New(caCert *x509.Certificate, caKey crypto.PrivateKey) (*TLSInterceptor, error) {
	return &TLSInterceptor{
		caCert: caCert,
		caKey:  caKey,
	}, nil
}

func NewFromFiles(cert string, key string) (*TLSInterceptor, error) {
	caCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	return New(caCert.Leaf, caCert.PrivateKey)
}

func (interceptor *TLSInterceptor) GenerateCertificate(host string) (*tls.Certificate, error) {
	hostKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject: pkix.Name{
			CommonName: host,
		},
		DNSNames:    []string{host},
		NotBefore:   now.Add(-time.Hour),
		NotAfter:    now.Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	hostCertDER, err := x509.CreateCertificate(rand.Reader, tmpl, interceptor.caCert,
		&hostKey.PublicKey, interceptor.caKey)
	if err != nil {
		return nil, err
	}

	hostKeyDER, err := x509.MarshalECPrivateKey(hostKey)
	if err != nil {
		return nil, err
	}

	hostCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: hostCertDER})
	hostKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "ECDSA PRIVATE KEY", Bytes: hostKeyDER})

	tlsCert, err := tls.X509KeyPair(hostCertPEM, hostKeyPEM)
	if err != nil {
		return nil, err
	}

	return &tlsCert, nil
}
