using App.Auth;

namespace App.Tests;

public class TokenServiceNUnitTests
{
    [TestCase("expired")]
    public void RejectsExpiredToken(string token)
    {
        var service = new TokenService("test");
        Assert.That(service.ValidateToken(token), Is.False);
    }
}

