package ca

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brianm/epithet/pkg/sshcert"
	log "github.com/sirupsen/logrus"
)

// CA performs CA operations
type CA struct {
	sshKeygen        string
	caPrivateKeyPath string
	publicKey        sshcert.RawPublicKey
}

// New creates a new CA
func New(publicKey sshcert.RawPublicKey, privateKey sshcert.RawPrivateKey) (*CA, error) {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return nil, fmt.Errorf("unable to find ssh-keygen: %w", err)
	}

	caPrivateKeyFile, err := ioutil.TempFile("", "cakey*")
	if err != nil {
		return nil, err
	}
	defer caPrivateKeyFile.Close()

	_, err = caPrivateKeyFile.WriteString(string(privateKey))
	if err != nil {
		return nil, fmt.Errorf("unable to write privkey to tempfile: %w", err)
	}
	caPrivateKeyFile.Close()

	ca := &CA{
		sshKeygen:        sshKeygen,
		caPrivateKeyPath: caPrivateKeyFile.Name(),
		publicKey:        publicKey,
	}

	return ca, nil
}

// Close closes down the CA
func (c *CA) Close() error {
	err := os.Remove(c.caPrivateKeyPath)
	c.sshKeygen = ""
	c.caPrivateKeyPath = ""
	c.publicKey = ""
	return err
}

// PublicKey returns the ssh on-disk format public key for the CA
func (c *CA) PublicKey() sshcert.RawPublicKey {
	return c.publicKey
}

// CertParams are options which can be set on a certificate
type CertParams struct {
	Identity   string
	Names      []string
	Expiration time.Duration
}

// SignPublicKey signs a key to generate a certificate
func (c *CA) SignPublicKey(pubkey sshcert.RawPublicKey, params *CertParams) (sshcert.RawCertificate, error) {
	// `ssh-keygen -s test/ca/ca -z 2 -V +15m -I brianm -n brianm,waffle ./id_ed25519.pub`

	pubkeyFile, err := ioutil.TempFile("", "id_*.pub")
	if err != nil {
		return "", err
	}
	defer os.Remove(pubkeyFile.Name())
	defer pubkeyFile.Close()
	_, err = pubkeyFile.WriteString(string(pubkey))
	if err != nil {
		return "", err
	}

	args := []string{"-s", c.caPrivateKeyPath, "-I", params.Identity}

	if params.Expiration != 0 {
		secs := int(params.Expiration.Seconds())
		interval := fmt.Sprintf("always:+%ds", secs)
		args = append(args, "-V", interval)
	}

	names := strings.Join(params.Names, ",")
	args = append(args, "-n", names)

	args = append(args, pubkeyFile.Name())
	log.Println(args)
	cmd := exec.Command(c.sshKeygen, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %w", string(out), err)
	}

	certPath := strings.ReplaceAll(pubkeyFile.Name(), ".pub", "-cert.pub")
	b, err := ioutil.ReadFile(certPath)
	if err != nil {
		return "", err
	}

	return sshcert.RawCertificate(string(b)), nil
}

// AuthToken is the token passed from the plugin through to
// the CA (and to the ca verifier plugin matching Provider)
// Token is opaque and can hold whatever the plugins need it to
type AuthToken struct {
	Provider string
	Token    string
}
