import { validateToken } from "../src/auth/token-validator";

describe("TokenService", () => {
    test("expired token is rejected", () => {
        expect(validateToken("expired")).toBe(false);
    });
    it("accepts a live token", () => { expect(validateToken("live")).toBe(true); });
});

