function base64UrlToUtf8(segment: string): string {
	if (!segment) {
		throw new Error("Invalid JWT: empty payload segment");
	}
	let base64 = segment.replace(/-/g, "+").replace(/_/g, "/");
	const pad = base64.length % 4;
	if (pad === 2) {
		base64 += "==";
	} else if (pad === 3) {
		base64 += "=";
	} else if (pad === 1) {
		throw new Error("Invalid JWT: malformed base64url in payload segment");
	}
	try {
		return atob(base64);
	} catch {
		throw new Error("Invalid JWT: payload segment is not valid base64");
	}
}

/**
 * Decodes the JWT payload (middle segment) as JSON.
 * Validates `header.payload.signature` shape before decoding so callers get clear errors.
 */
export function decodeJwtPayload(token: string): Record<string, unknown> {
	if (typeof token !== "string" || !token.trim()) {
		throw new Error("Invalid JWT: token must be a non-empty string");
	}
	const parts = token.trim().split(".");
	if (parts.length !== 3) {
		throw new Error(
			`Invalid JWT: expected header.payload.signature (3 segments), got ${parts.length}`,
		);
	}
	const json = base64UrlToUtf8(parts[1]);
	let parsed: unknown;
	try {
		parsed = JSON.parse(json);
	} catch {
		throw new Error("Invalid JWT: payload is not valid JSON");
	}
	if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
		throw new Error("Invalid JWT: payload must be a JSON object");
	}
	return parsed as Record<string, unknown>;
}

export function extractTenantId(accessToken: string): string | null {
	try {
		const payload = decodeJwtPayload(accessToken);
		const raw = payload.tenant_id ?? payload.tenantId;
		if (typeof raw === "string" && raw.length > 0) return raw;
		if (typeof raw === "number") return String(raw);
	} catch (err) {
		if (process.env.NODE_ENV === "development") {
			const message = err instanceof Error ? err.message : String(err);
			console.warn(
				"[extractTenantId] could not read tenant_id from access token:",
				message,
			);
		}
	}
	return null;
}
