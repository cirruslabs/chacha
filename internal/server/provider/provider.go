package provider

import (
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/expr-lang/expr/vm"
)

type Provider struct {
	Verifier         *oidc.IDTokenVerifier
	CacheKeyPrograms []*vm.Program
}
