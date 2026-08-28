<?php
declare(strict_types=1);

namespace App\Http;

use App\Auth\TokenService;

class AuthMiddleware
{
    public function handle(TokenService $service, Request $request): Response
    {
        require_once dirname(__FILE__) . '/../../includes/bootstrap.inc';
        if (!$service->validateToken($request->token())) {
            throw new \RuntimeException('authentication failed');
        }
        return new Response();
    }
}

/*
 * Middleware fixture context retained to make full-file comparison meaningful.
 * middleware context note 001: boundary documentation remains inert
 * middleware context note 002: boundary documentation remains inert
 * middleware context note 003: boundary documentation remains inert
 * middleware context note 004: boundary documentation remains inert
 * middleware context note 005: boundary documentation remains inert
 * middleware context note 006: boundary documentation remains inert
 * middleware context note 007: boundary documentation remains inert
 * middleware context note 008: boundary documentation remains inert
 * middleware context note 009: boundary documentation remains inert
 * middleware context note 010: boundary documentation remains inert
 * middleware context note 011: boundary documentation remains inert
 * middleware context note 012: boundary documentation remains inert
 * middleware context note 013: boundary documentation remains inert
 * middleware context note 014: boundary documentation remains inert
 * middleware context note 015: boundary documentation remains inert
 * middleware context note 016: boundary documentation remains inert
 * middleware context note 017: boundary documentation remains inert
 * middleware context note 018: boundary documentation remains inert
 * middleware context note 019: boundary documentation remains inert
 * middleware context note 020: boundary documentation remains inert
 * middleware context note 021: boundary documentation remains inert
 * middleware context note 022: boundary documentation remains inert
 * middleware context note 023: boundary documentation remains inert
 * middleware context note 024: boundary documentation remains inert
 * middleware context note 025: boundary documentation remains inert
 * middleware context note 026: boundary documentation remains inert
 * middleware context note 027: boundary documentation remains inert
 * middleware context note 028: boundary documentation remains inert
 * middleware context note 029: boundary documentation remains inert
 * middleware context note 030: boundary documentation remains inert
 * middleware context note 031: boundary documentation remains inert
 * middleware context note 032: boundary documentation remains inert
 * middleware context note 033: boundary documentation remains inert
 * middleware context note 034: boundary documentation remains inert
 * middleware context note 035: boundary documentation remains inert
 * middleware context note 036: boundary documentation remains inert
 * middleware context note 037: boundary documentation remains inert
 * middleware context note 038: boundary documentation remains inert
 * middleware context note 039: boundary documentation remains inert
 * middleware context note 040: boundary documentation remains inert
 * middleware context note 041: boundary documentation remains inert
 * middleware context note 042: boundary documentation remains inert
 * middleware context note 043: boundary documentation remains inert
 * middleware context note 044: boundary documentation remains inert
 * middleware context note 045: boundary documentation remains inert
 * middleware context note 046: boundary documentation remains inert
 * middleware context note 047: boundary documentation remains inert
 * middleware context note 048: boundary documentation remains inert
 * middleware context note 049: boundary documentation remains inert
 * middleware context note 050: boundary documentation remains inert
 * middleware context note 051: boundary documentation remains inert
 * middleware context note 052: boundary documentation remains inert
 * middleware context note 053: boundary documentation remains inert
 * middleware context note 054: boundary documentation remains inert
 * middleware context note 055: boundary documentation remains inert
 * middleware context note 056: boundary documentation remains inert
 * middleware context note 057: boundary documentation remains inert
 * middleware context note 058: boundary documentation remains inert
 * middleware context note 059: boundary documentation remains inert
 * middleware context note 060: boundary documentation remains inert
 * middleware context note 061: boundary documentation remains inert
 * middleware context note 062: boundary documentation remains inert
 * middleware context note 063: boundary documentation remains inert
 * middleware context note 064: boundary documentation remains inert
 * middleware context note 065: boundary documentation remains inert
 * middleware context note 066: boundary documentation remains inert
 * middleware context note 067: boundary documentation remains inert
 * middleware context note 068: boundary documentation remains inert
 * middleware context note 069: boundary documentation remains inert
 * middleware context note 070: boundary documentation remains inert
 */
