package steamtotp_test

import (
	"fmt"
	"log"

	"github.com/k64z/steamstacks/steamtotp"
)

func ExampleGenerateAuthCode() {
	// The shared secret is the base64 shared_secret field of a mobile
	// authenticator. This one is a documentation placeholder.
	code, err := steamtotp.GenerateAuthCode("REtoaXhpazJTcTJqNXFTdg==", 0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(code)) // codes are always 5 characters
	// Output: 5
}

func ExampleGetDeviceID() {
	fmt.Println(steamtotp.GetDeviceID(76561198000000000))
}
