using App.Auth;

namespace App.Tests;

public class TokenServiceXunitTests
{
    [Fact]
    public void RejectsExpiredToken()
    {
        var service = new TokenService("test");
        Assert.False(service.ValidateToken("expired"));
    }
}

