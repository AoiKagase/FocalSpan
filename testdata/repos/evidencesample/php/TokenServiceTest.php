<?php
namespace Evidence\Tests;

final class TokenServiceTest extends TestCase
{
    public function testExpiredTokenIsRejected(): void
    {
        self::assertFalse((new TokenService())->validateToken('expired'));
    }
}
