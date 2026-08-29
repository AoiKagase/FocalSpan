export const normalizeToken = (value: string): string => value.trim();
export function splitToken(value: string) { return value.split("."); }
const tokenPattern = /\{token\}/gi;

