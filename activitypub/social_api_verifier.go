import (
	"net/http"
	"net/url"
)

func (s socialAPIVerifier) Verify(r *http.Request) (authenticatedUser *url.URL, authn bool, authz bool, err error) {
	panic("not implemented")
}

func (s socialAPIVerifier) VerifyForOutbox(r *http.Request, outbox *url.URL) (authn bool, authz bool, err error) {
	panic("not implemented")
}

