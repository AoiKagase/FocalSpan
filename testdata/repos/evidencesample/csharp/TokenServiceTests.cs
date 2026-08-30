namespace Evidence.Tests;

public sealed class TokenServiceTests
{
    [Fact]
    public void RejectsExpiredEvidenceToken()
    {
        Assert.False(new TokenService().ValidateCSharpEvidenceToken("expired"));
    }
}
