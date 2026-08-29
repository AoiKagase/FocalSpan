namespace App.Auth;

public partial class TokenService
{
    public bool ValidateForHeader(string token) => ValidateToken(token);
}

