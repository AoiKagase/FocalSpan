import { authenticateEvidenceRequest } from "./token-service";

export const evidenceMiddleware = (token: string): boolean =>
  authenticateEvidenceRequest(token);
