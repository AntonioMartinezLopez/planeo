// auth.d.ts
declare module '#auth-utils' {
  interface User {
    // Add your own fields
    email_verified: boolean
    name: string
    groups: string[]
    preferred_username: string
    given_name: string
    family_name: string
    email: string
  }

  interface UserSession {
    // Add your own fields
    user: User
    tokens: {
      access_token: string
    }
  }

  interface SecureSessionData {
    // Add your own fields
    refresh_token: string
  }
}

export { }
