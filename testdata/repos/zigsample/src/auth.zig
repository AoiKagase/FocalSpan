const std = @import("std");
const AuthState = enum { logged_out, logged_in };
const Credentials = struct { user: []const u8 };
const AuthError = union(enum) { invalid_token, expired };
const Handle = opaque {};

fn normalizeToken(token: []const u8) []const u8 {
    return token;
}

pub fn validateToken(token: []const u8) !bool {
    return normalizeToken(token).len > 0;
}

pub export fn pluginEntry() void {
    _ = validateToken("guest");
}

comptime {
    _ = @TypeOf(Credentials);
}

test "expired token" {
    try std.testing.expect(!try validateToken("expired"));
}
