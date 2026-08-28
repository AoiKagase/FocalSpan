<?php
namespace App\Auth;

interface TokenValidatorContract
{
    public function validateToken(Token $token): bool;
}
