import TokenService from "../auth/token-service.js";
export { TokenService };
export async function bootstrap(token) {
    const service = new TokenService("secret");
    return service.validateToken(token);
}

