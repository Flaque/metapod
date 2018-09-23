package httpsig

import (
	"crypto"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

var _ Verifier = &verifier{}

type verifier struct {
	header      http.Header
	kId         string
	signature   string
	headers     []string
	sigStringFn func(http.Header, []string) (string, error)
}

func newVerifier(h http.Header, sigStringFn func(http.Header, []string) (string, error)) (*verifier, error) {
	s, err := getSignatureScheme(h)
	if err != nil {
		return nil, err
	}
	kId, sig, headers, err := getSignatureComponents(s)
	if err != nil {
		return nil, err
	}
	return &verifier{
		header:      h,
		kId:         kId,
		signature:   sig,
		headers:     headers,
		sigStringFn: sigStringFn,
	}, nil
}

func (v *verifier) KeyId() string {
	return v.kId
}

func (v *verifier) Verify(pKey crypto.PublicKey, algo Algorithm) error {
	s, err := signerFromString(string(algo))
	if err == nil {
		return v.asymmVerify(s, pKey)
	}
	m, err := macerFromString(string(algo))
	if err == nil {
		return v.macVerify(m, pKey)
	}
	return fmt.Errorf("no crypto implementation available for %q", algo)
}

func (v *verifier) macVerify(m macer, pKey crypto.PublicKey) error {
	key, ok := pKey.([]byte)
	if !ok {
		return fmt.Errorf("public key for MAC verifying must be of type []byte")
	}
	signature, err := v.sigStringFn(v.header, v.headers)
	if err != nil {
		return err
	}
	actualMAC, err := base64.StdEncoding.DecodeString(v.signature)
	if err != nil {
		return err
	}
	ok, err = m.Equal([]byte(signature), actualMAC, key)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("invalid http signature")
	}
	return nil
}

func (v *verifier) asymmVerify(s signer, pKey crypto.PublicKey) error {
	toHash, err := v.sigStringFn(v.header, v.headers)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(v.signature)
	if err != nil {
		return err
	}
	err = s.Verify(pKey, []byte(toHash), signature)
	if err != nil {
		return err
	}
	return nil
}

func getSignatureScheme(h http.Header) (string, error) {
	s := h.Get(string(Signature))
	sigHasAll := strings.Contains(s, keyIdParameter) ||
		strings.Contains(s, headersParameter) ||
		strings.Contains(s, signatureParameter)
	a := h.Get(string(Authorization))
	authHasAll := strings.Contains(a, keyIdParameter) ||
		strings.Contains(a, headersParameter) ||
		strings.Contains(a, signatureParameter)
	if sigHasAll && authHasAll {
		return "", fmt.Errorf("both %q and %q have signature parameters", Signature, Authorization)
	} else if !sigHasAll && !authHasAll {
		return "", fmt.Errorf("neither %q nor %q have signature parameters", Signature, Authorization)
	} else if sigHasAll {
		return s, nil
	} else { // authHasAll
		return a, nil
	}
}

func getSignatureComponents(s string) (kId, sig string, headers []string, err error) {
	params := strings.Split(s, parameterSeparater)
	for _, p := range params {
		kv := strings.SplitN(p, parameterKVSeparater, 2)
		if len(kv) != 2 {
			err = fmt.Errorf("malformed http signature parameter: %v", kv)
			return
		}
		k := kv[0]
		v := strings.Trim(kv[1], parameterValueDelimiter)
		switch k {
		case keyIdParameter:
			kId = v
		case algorithmParameter:
			// Deprecated, ignore
		case headersParameter:
			headers = strings.Split(v, headerParameterValueDelim)
		case signatureParameter:
			sig = v
		default:
			// Ignore unrecognized parameters
		}
	}
	if len(kId) == 0 {
		err = fmt.Errorf("missing %q parameter in http signature", keyIdParameter)
	} else if len(sig) == 0 {
		err = fmt.Errorf("missing %q parameter in http signature", signatureParameter)
	} else if len(headers) == 0 { // Optional
		headers = defaultHeaders
	}
	return
}
