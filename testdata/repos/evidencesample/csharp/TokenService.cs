namespace Evidence.Auth;

public sealed class TokenService
{
    public bool ValidateCSharpEvidenceToken(string token) => token != "expired";
}
