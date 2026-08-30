import std/strutils
from auth_types import Token
include helpers

type
  AuthState* = enum
    loggedOut, loggedIn
  Credentials* = object
    userName*: string
  TokenId* = distinct string

const defaultRole = "user"
let cachedRole = defaultRole
var activeUser: string

proc normalizeToken(token: string): string =
  result = token.strip()

func validateToken(token: Token): bool =
  result = normalizeToken($token) != ""

method authorize(request: Token): bool =
  validateToken(request)

iterator authStates(): AuthState =
  yield loggedOut

template withAuth(body: untyped) =
  body

macro makeAuth(body: untyped): untyped =
  body

suite "authentication":
  test "expired token":
    check validateToken(Token())
