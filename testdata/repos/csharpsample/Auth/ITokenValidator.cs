namespace App.Auth;

public interface ITokenValidator
{
    bool ValidateToken(string token);
}

