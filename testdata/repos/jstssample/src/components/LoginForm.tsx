import { validateToken } from "../auth/token-validator";

export function LoginForm() {
    return <form data-valid={validateToken("token")} />;
}
