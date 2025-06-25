import { jwtDecode } from "jwt-decode";

export function isTokenExpired(token: string): boolean {
  const decoded = jwtDecode(token);
  const expiresAt = (decoded?.exp ?? 0) * 1000;

  return Date.now() >= expiresAt;
}
