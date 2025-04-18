package keycloak

import (
	"encoding/json"
)

type DefaultRole int

const (
	User DefaultRole = iota
	Planner
	Admin
)

var roles = [...]string{"User", "Planner", "Admin"}

func (role DefaultRole) String() string {
	return roles[role]
}

func (role *DefaultRole) FromString(roleName string) DefaultRole {
	return map[string]DefaultRole{
		"User":    User,
		"Planner": Planner,
		"Admin":   Admin,
	}[roleName]
}

func (role DefaultRole) MarshalJSON() ([]byte, error) {
	return json.Marshal(role.String())
}

func (role *DefaultRole) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	*role = role.FromString(s)
	return nil
}
