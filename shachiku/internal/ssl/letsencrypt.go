package ssl

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "shachiku/internal/config"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u *MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func GetPublicIP() string {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Println("ssl: Failed to get public IP:", err)
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("ssl: Failed to read public IP:", err)
		return ""
	}
	return strings.TrimSpace(string(body))
}

func ApplyCertificate(domain, email string) error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	myUser := MyUser{
		Email: email,
		key:   privateKey,
	}

	config := lego.NewConfig(&myUser)

	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}

	// Use HTTP-01 challenge provider server listening on port 80
	err = client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
	if err != nil {
		return err
	}

	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return err
	}
	myUser.Registration = reg

	var certificates *certificate.Resource

	ipAddr := net.ParseIP(domain)
	if ipAddr != nil {
		// For IPs, we must NOT put the IP in CommonName.
		// We generate a CSR manually with the IP in SANs (IPAddresses).
		// Note: The certificate key MUST be different from the account key.
		certPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}

		csrTemplate := &x509.CertificateRequest{
			IPAddresses: []net.IP{ipAddr},
		}
		csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, certPrivateKey)
		if err != nil {
			return err
		}

		csr, err := x509.ParseCertificateRequest(csrBytes)
		if err != nil {
			return err
		}

		request := certificate.ObtainForCSRRequest{
			CSR:     csr,
			Bundle:  true,
			Profile: "shortlived",
		}
		certificates, err = client.Certificate.ObtainForCSR(request)
		if err != nil {
			return err
		}

		encodedKey, err := x509.MarshalECPrivateKey(certPrivateKey)
		if err != nil {
			return err
		}
		certificates.PrivateKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encodedKey})
	} else {
		request := certificate.ObtainRequest{
			Domains: []string{domain},
			Bundle:  true,
			Profile: "shortlived",
		}
		certificates, err = client.Certificate.Obtain(request)
		if err != nil {
			return err
		}
	}

	// ensure data dir exists
	os.MkdirAll(appconfig.GetDataDir(), 0755)

	err = os.WriteFile(filepath.Join(appconfig.GetDataDir(), "certificate.crt"), certificates.Certificate, 0644)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(appconfig.GetDataDir(), "private.key"), certificates.PrivateKey, 0600)
	if err != nil {
		return err
	}

	return nil
}

// InitCertificate checks if IS_PUBLIC is true, if so gets public IP and applies for a cert.
func InitCertificate() {
	if os.Getenv("IS_PUBLIC") == "" {
		return
	}

	go StartCertificateRenewalLoop()

	// Skip if cert already exists to avoid redundant calls to Let's Encrypt
	certPath := filepath.Join(appconfig.GetDataDir(), "certificate.crt")
	keyPath := filepath.Join(appconfig.GetDataDir(), "private.key")
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			log.Println("ssl: Certificate already exists in data directory, skipping generation")
			return
		}
	}

	ip := GetPublicIP()
	if ip == "" {
		log.Println("ssl: Could not determine public IP, skipping certificate generation")
		return
	}

	email := os.Getenv("LETSENCRYPT_EMAIL")
	if email == "" {
		email = "admin@" + strings.ReplaceAll(ip, ".", "-") + ".com" // Provide a valid fallback email
	}

	log.Println("ssl: IP determined as", ip, "- Applying for certificate...")
	if err := ApplyCertificate(ip, email); err != nil {
		log.Println("ssl: Failed to apply IP certificate:", err)
	} else {
		log.Println("ssl: Successfully applied for IP certificate for", ip)
	}
}

func StartCertificateRenewalLoop() {
	time.Sleep(1 * time.Minute)
	CheckAndRenewCertificate()

	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		CheckAndRenewCertificate()
	}
}

func CheckAndRenewCertificate() {
	certFile := filepath.Join(appconfig.GetDataDir(), "certificate.crt")
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Println("ssl: Failed to parse certificate:", err)
		return
	}

	// Check if certificate expires in less than 24 hours
	if time.Until(cert.NotAfter) < 24*time.Hour {
		log.Println("ssl: Certificate expires in less than 24 hours. Attempting renewal...")

		ip := GetPublicIP()
		if ip == "" {
			log.Println("ssl: Could not determine public IP for renewal")
			return
		}

		email := os.Getenv("LETSENCRYPT_EMAIL")
		if email == "" {
			email = "admin@" + strings.ReplaceAll(ip, ".", "-") + ".com"
		}

		if err := ApplyCertificate(ip, email); err != nil {
			log.Println("ssl: Failed to renew certificate:", err)
		} else {
			log.Println("ssl: Certificate renewed successfully. Exiting to trigger restart.")
			os.Exit(0)
		}
	}
}
