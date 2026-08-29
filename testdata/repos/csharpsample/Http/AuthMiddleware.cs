using App.Auth;

namespace App.Http
{
    public sealed class AuthMiddleware
    {
        public bool Handle(TokenService service, string token)
        {
            return service.ValidateToken(token);
        }
    }
}

