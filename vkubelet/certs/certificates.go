package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"time"
)

const (
	validFor = 365 * 24 * time.Hour 
	keySize  = 4096
)

// this is based on https://golang.org/src/crypto/tls/generate_cert.go and
// the existing script createCertAndKey.sh
func GenerateCertKeyPair() (string, string, error) {

	certFileName := "cert.pem"
	keyFileName := "key.pem"

	priv, err := rsa.GenerateKey(rand.Reader, keySize)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	notBefore := time.Now().Add(-1 * 24 * time.Hour)
	notAfter := notBefore.Add(validFor)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country: []string{"US"},
			Province: []string{"CA"},
			Locality: []string{"virtualkubelet"},
			OrganizationalUnit: []string{"virtualkubelet"},
			Organization: []string{"virtualkubelet"},
			CommonName: "virtualkubelet",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader,
		&template,
		&template,
		&priv.PublicKey,
		priv,
	)
	if err != nil {
		log.Printf("Failed to create certificate: %s\n", err)
		return "", "", err
	}
	certOut, err := os.Create("cert.pem")
	if err != nil {
		log.Printf("failed to open cert.pem for writing: %s\n", err)
		return "", "", err
	}
	defer certOut.Close()
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		log.Printf("failed to PEM encode certificate: %s\n", err)
		return "", "", err
	}
	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("failed to open key.pem for writing: %s\n", err)
		return "", "", err
	}
	defer keyOut.Close()
	p := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	err = pem.Encode(keyOut, p)
	if err != nil {
		log.Printf("failed to PEM encode key: %s\n", err)
		return "", "", err
	}
	return certFileName, keyFileName, nil
}
