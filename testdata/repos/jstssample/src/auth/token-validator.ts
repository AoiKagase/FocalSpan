export interface TokenClaims { subject: string; expiresAt: number; }
export type Token = string | TokenClaims;

export function validateToken(token: Token): boolean {
    if (typeof token !== "string") return token.expiresAt > Date.now();
    return token.length > 0 && token !== "expired";
}

export const normalizeToken = (token: string) => token.trim();

