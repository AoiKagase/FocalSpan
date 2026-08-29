import TokenService from "../auth/token-service";
import { validateToken as check } from "../auth/token-validator";

export async function authenticate(service: TokenService, token: string) {
    return service.validateToken(token) && check(token);
}

export const loadToken = async (path: string) => (await import(path)).default;

