package httpsig

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
)

const (
	// Signature Parameters
	keyIdParameter            = "keyId"
	algorithmParameter        = "algorithm"
	headersParameter          = "headers"
	signatureParameter        = "signature"
	parameterKVSeparater      = "="
	parameterValueDelimiter   = "\""
	parameterSeparater        = ","
	headerParameterValueDelim = " "
	// RequestTarget specifies to include the http request method and
	// entire URI in the signature. Pass it as a header to NewSigner.
	RequestTarget = "(request-target)"
	dateHeader    = "date"

	// Signature String Construction
	headerFieldDelimiter   = ": "
	headersDelimiter       = "\n"
	headerValueDelimiter   = ", "
	requestTargetSeparator = " "
)

var defaultHeaders = []string{dateHeader}

var _ Signer = &macSigner{}

type macSigner struct {
	m            macer
	headers      []string
	targetHeader SignatureScheme
}

func (m *macSigner) SignRequest(pKey crypto.PrivateKey, pubKeyId string, r *http.Request) error {
	s, err := m.signatureString(r)
	if err != nil {
		return err
	}
	enc, err := m.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header, string(m.targetHeader), pubKeyId, m.m.String(), enc, m.headers)
	return nil
}

func (m *macSigner) SignResponse(pKey crypto.PrivateKey, pubKeyId string, r http.ResponseWriter) error {
	s, err := m.signatureStringResponse(r)
	if err != nil {
		return err
	}
	enc, err := m.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header(), string(m.targetHeader), pubKeyId, m.m.String(), enc, m.headers)
	return nil
}

func (m *macSigner) signSignature(pKey crypto.PrivateKey, s string) (string, error) {
	pKeyBytes, ok := pKey.([]byte)
	if !ok {
		return "", fmt.Errorf("private key for MAC signing must be of type []byte")
	}
	sig, err := m.m.Sign([]byte(s), pKeyBytes)
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(sig)
	return enc, nil
}

func (m *macSigner) signatureString(r *http.Request) (string, error) {
	return signatureString(r.Header, m.headers, addRequestTarget(r))
}

func (m *macSigner) signatureStringResponse(r http.ResponseWriter) (string, error) {
	return signatureString(r.Header(), m.headers, requestTargetNotPermitted)
}

var _ Signer = &asymmSigner{}

type asymmSigner struct {
	s            signer
	headers      []string
	targetHeader SignatureScheme
}

func (a *asymmSigner) SignRequest(pKey crypto.PrivateKey, pubKeyId string, r *http.Request) error {
	s, err := a.signatureString(r)
	if err != nil {
		return err
	}
	enc, err := a.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header, string(a.targetHeader), pubKeyId, a.s.String(), enc, a.headers)
	return nil
}

func (a *asymmSigner) SignResponse(pKey crypto.PrivateKey, pubKeyId string, r http.ResponseWriter) error {
	s, err := a.signatureStringResponse(r)
	if err != nil {
		return err
	}
	enc, err := a.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header(), string(a.targetHeader), pubKeyId, a.s.String(), enc, a.headers)
	return nil
}

func (a *asymmSigner) signSignature(pKey crypto.PrivateKey, s string) (string, error) {
	sig, err := a.s.Sign(rand.Reader, pKey, []byte(s))
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(sig)
	return enc, nil
}

func (a *asymmSigner) signatureString(r *http.Request) (string, error) {
	return signatureString(r.Header, a.headers, addRequestTarget(r))
}

func (a *asymmSigner) signatureStringResponse(r http.ResponseWriter) (string, error) {
	return signatureString(r.Header(), a.headers, requestTargetNotPermitted)
}

func setSignatureHeader(h http.Header, targetHeader, pubKeyId, algo, enc string, headers []string) {
	if len(headers) == 0 {
		headers = defaultHeaders
	}
	var b bytes.Buffer
	// KeyId
	b.WriteString(keyIdParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeyId)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Algorithm (deprecated)
	// TODO: Remove this.
	b.WriteString(algorithmParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(algo)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Headers
	b.WriteString(headersParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	for i, h := range headers {
		b.WriteString(strings.ToLower(h))
		if i != len(headers)-1 {
			b.WriteString(headerParameterValueDelim)
		}
	}
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Signature
	b.WriteString(signatureParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(enc)
	b.WriteString(parameterValueDelimiter)
	h.Add(targetHeader, b.String())
}

func requestTargetNotPermitted(b bytes.Buffer) error {
	return fmt.Errorf("cannot sign with %q on anything other than an http request", RequestTarget)
}

func addRequestTarget(r *http.Request) func(b bytes.Buffer) error {
	return func(b bytes.Buffer) error {
		b.WriteString(RequestTarget)
		b.WriteString(headerFieldDelimiter)
		b.WriteString(strings.ToLower(r.Method))
		b.WriteString(requestTargetSeparator)
		b.WriteString(r.URL.String())
		return nil
	}
}

func signatureString(values http.Header, include []string, requestTargetFn func(b bytes.Buffer) error) (string, error) {
	if len(include) == 0 {
		include = defaultHeaders
	}
	var b bytes.Buffer
	for n, i := range include {
		i := strings.ToLower(i)
		if i == RequestTarget {
			err := requestTargetFn(b)
			if err != nil {
				return "", err
			}
		} else {
			hv, ok := values[textproto.CanonicalMIMEHeaderKey(i)]
			if !ok {
				return "", fmt.Errorf("missing header %q", i)
			}
			b.WriteString(i)
			b.WriteString(headerFieldDelimiter)
			for i, v := range hv {
				b.WriteString(strings.TrimSpace(v))
				if i < len(hv)-1 {
					b.WriteString(headerValueDelimiter)
				}
			}
		}
		if n < len(include)-1 {
			b.WriteString(headersDelimiter)
		}
	}
	return b.String(), nil
}
