// Package surface owns the identity and authority types of host integration
// surfaces. Each client adapter declares its own surface identities.
package surface

// ID identifies a host integration surface.
type ID string

// Authority identifies the system that owns a surface.
type Authority string

// AuthorityAIGW identifies a surface whose bounded projection is owned by AIGW.
const AuthorityAIGW Authority = "aigw"
