package keycloak

type AdminKeycloakSession struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type KeycloakUser struct {
	Id              string   `json:"id" example:"123456" doc:"User identifier within Keycloak" validate:"required"`
	Userame         string   `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string   `json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName        string   `json:"lastName" validate:"required"`
	Email           string   `json:"email" validate:"required"`
	Enabled         bool     `json:"enabled"`
	Totp            bool     `json:"totp"`
	RequiredActions []string `json:"requiredActions" validate:"required"`
}

type KeycloakGroup struct {
	Id   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
	Path string `json:"path" validate:"required"`
}

type KeycloakClient struct {
	Uuid     string `json:"id" validate:"required"`
	ClientId string `json:"clientId" validate:"reqiured"`
}

type KeycloakClientRole struct {
	Id   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

type DefaultRole int

const (
	User DefaultRole = iota
	Planner
	Admin
)

var roles = [...]string{"User", "Planner", "Admin"}

func (me DefaultRole) String() string {
	return roles[me]
}
