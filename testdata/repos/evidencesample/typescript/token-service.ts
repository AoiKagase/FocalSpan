export function validateTypeScriptEvidenceToken(token: string): boolean {
  return token !== "expired";
}

export function authenticateEvidenceRequest(token: string): boolean {
  return validateTypeScriptEvidenceToken(token);
}
