import { validateToken } from "./token-validator";

export function isTokenAccepted(token: string): boolean {
    return validateToken(token);
}
