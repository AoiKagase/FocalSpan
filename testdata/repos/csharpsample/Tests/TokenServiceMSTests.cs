using App.Auth;

namespace App.Tests;

[TestClass]
public class TokenServiceMSTests
{
    [TestMethod]
    public void RejectsExpiredToken()
    {
        var service = new TokenService("test");
        Microsoft.VisualStudio.TestTools.UnitTesting.Assert.IsFalse(service.ValidateToken("expired"));
    }
}

