package certs_test

import (
	"fmt"

	"github.com/gxmmx/compage-go/certs"
)

func ExampleNewCSR() {
	bundle, err := certs.NewCSR(certs.WithSubject(certs.Identity{Organization: "Example", CommonName: "device-1"}), certs.WithSANs(certs.SANs{DNSNames: []string{"device-1.example.test"}}), certs.WithKeySpec(certs.Ed25519))
	if err != nil {
		panic(err)
	}
	fmt.Println(bundle.CSR() != nil, bundle.Key() != nil)
	// Output: true true
}
