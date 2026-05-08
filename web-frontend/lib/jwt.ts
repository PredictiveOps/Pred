export function decodeJwtPayload(token: string): Record<string, unknown> {
	const [, payload] = token.split(".");
	return JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
}

export function extractTenantId(accessToken: string): string | null {
	try {
		const payload = decodeJwtPayload(accessToken);
		const raw = payload.tenant_id ?? payload.tenantId;
		if (typeof raw === "string" && raw.length > 0) return raw;
		if (typeof raw === "number") return String(raw);
	} catch {}
	return null;
}
