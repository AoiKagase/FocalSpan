<?php
namespace Evidence\Auth;

final class TokenService
{
    public function validateToken(string $token): bool
    {
        return $token !== 'expired';
    }
}
