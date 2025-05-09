package utils

import (
	"fmt"
	"os"
	"testing"
)

func TestCertificate(t *testing.T) {
	privKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Create a subject
	subject := CertificateSubject(
		"some.domain.com",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	// Create a certificate request
	csr, err := GenerateCsr(subject, privKey, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// create a certificate
	cert, err := GenerateSelfsignedCertificate(csr, privKey, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// fmt.Println("Certificate", cert.Subject.CommonName)

	pem := EncodeCrtToPem(cert, false, false)
	// write to file
	err = os.WriteFile("test.crt", []byte(pem), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Printf("%s", pem)
}
