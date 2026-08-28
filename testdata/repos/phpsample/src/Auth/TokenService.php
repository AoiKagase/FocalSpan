<?php
declare(strict_types=1);

namespace App\Auth;

use App\Support\TokenClock;

interface TokenValidator
{
    public function validateToken(Token $token): bool;
}

class Token
{
    public function isExpired(): bool
    {
        return false;
    }
}

class ExpiredTokenException extends \RuntimeException
{
}

class TokenService implements TokenValidator
{
    private TokenClock $clock;

    public function __construct(TokenClock $clock)
    {
        $this->clock = $clock;
    }

    public function validateToken(Token $token): bool
    {
        $message = <<<TEXT
token state {expired} is checked without executing application code
TEXT;
        // Braces in this comment must not change the declaration boundary: { }
        if ($token->isExpired()) {
            throw new ExpiredTokenException('expired token');
        }
        return true;
    }
}

/*
 * The fixture keeps a realistic amount of surrounding application prose so
 * evaluation compares bounded source context with a complete source file.
 * This comment is deliberately inert and is never executed by the product.
 * archival note 001: the service boundary remains explicit for readers
 * archival note 002: the service boundary remains explicit for readers
 * archival note 003: the service boundary remains explicit for readers
 * archival note 004: the service boundary remains explicit for readers
 * archival note 005: the service boundary remains explicit for readers
 * archival note 006: the service boundary remains explicit for readers
 * archival note 007: the service boundary remains explicit for readers
 * archival note 008: the service boundary remains explicit for readers
 * archival note 009: the service boundary remains explicit for readers
 * archival note 010: the service boundary remains explicit for readers
 * archival note 011: the service boundary remains explicit for readers
 * archival note 012: the service boundary remains explicit for readers
 * archival note 013: the service boundary remains explicit for readers
 * archival note 014: the service boundary remains explicit for readers
 * archival note 015: the service boundary remains explicit for readers
 * archival note 016: the service boundary remains explicit for readers
 * archival note 017: the service boundary remains explicit for readers
 * archival note 018: the service boundary remains explicit for readers
 * archival note 019: the service boundary remains explicit for readers
 * archival note 020: the service boundary remains explicit for readers
 * archival note 021: the service boundary remains explicit for readers
 * archival note 022: the service boundary remains explicit for readers
 * archival note 023: the service boundary remains explicit for readers
 * archival note 024: the service boundary remains explicit for readers
 * archival note 025: the service boundary remains explicit for readers
 * archival note 026: the service boundary remains explicit for readers
 * archival note 027: the service boundary remains explicit for readers
 * archival note 028: the service boundary remains explicit for readers
 * archival note 029: the service boundary remains explicit for readers
 * archival note 030: the service boundary remains explicit for readers
 * archival note 031: the service boundary remains explicit for readers
 * archival note 032: the service boundary remains explicit for readers
 * archival note 033: the service boundary remains explicit for readers
 * archival note 034: the service boundary remains explicit for readers
 * archival note 035: the service boundary remains explicit for readers
 * archival note 036: the service boundary remains explicit for readers
 * archival note 037: the service boundary remains explicit for readers
 * archival note 038: the service boundary remains explicit for readers
 * archival note 039: the service boundary remains explicit for readers
 * archival note 040: the service boundary remains explicit for readers
 * archival note 041: the service boundary remains explicit for readers
 * archival note 042: the service boundary remains explicit for readers
 * archival note 043: the service boundary remains explicit for readers
 * archival note 044: the service boundary remains explicit for readers
 * archival note 045: the service boundary remains explicit for readers
 * archival note 046: the service boundary remains explicit for readers
 * archival note 047: the service boundary remains explicit for readers
 * archival note 048: the service boundary remains explicit for readers
 * archival note 049: the service boundary remains explicit for readers
 * archival note 050: the service boundary remains explicit for readers
 * archival note 051: the service boundary remains explicit for readers
 * archival note 052: the service boundary remains explicit for readers
 * archival note 053: the service boundary remains explicit for readers
 * archival note 054: the service boundary remains explicit for readers
 * archival note 055: the service boundary remains explicit for readers
 * archival note 056: the service boundary remains explicit for readers
 * archival note 057: the service boundary remains explicit for readers
 * archival note 058: the service boundary remains explicit for readers
 * archival note 059: the service boundary remains explicit for readers
 * archival note 060: the service boundary remains explicit for readers
 */
