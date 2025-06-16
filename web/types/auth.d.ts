// auth.d.ts
declare module "#auth-utils" {
  interface User {
    email_verified: boolean;
    name: string;
    groups: string[];
    preferred_username: string;
    given_name: string;
    family_name: string;
    email: string;
  }

  interface UserSession {
    user: User;
  }

  interface SecureSessionData {
    refresh_token: string;
    access_token: string;
  }
}

export { };
