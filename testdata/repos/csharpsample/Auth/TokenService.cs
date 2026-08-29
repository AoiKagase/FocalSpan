using System;

namespace App.Auth;

public partial class TokenService(string key) : ITokenValidator
{
    [Fact]
    public bool ValidateToken(string token)
    {
        return !IsExpired(token) && Helper(token);
    }

    private bool IsExpired(string token) => token == "expired";
    public bool Helper(string token) => token.Length > 0;
    public string Label => $"token: {key}";
    public string this[int index] { get => key; set { key = value; } }
    public event EventHandler Changed;
}

