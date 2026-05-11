const BASE_URL = `${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8000"}/api/events`;

type AuthHeaders = Record<string, string>;

export type RawEvent = {
	id: number;
	tenant_id: string;
	payload: Record<string, unknown>;
	created_at: string;
};

export type RawEventsResponse = {
	count: number;
	events: RawEvent[];
};

function authHeaders(accessToken?: string, tenantId?: string): AuthHeaders {
	return {
		"Content-Type": "application/json",
		...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
		...(tenantId ? { "X-Tenant-Id": tenantId } : {}),
	};
}

async function handleResponse<T>(res: Response): Promise<T> {
	if (!res.ok) {
		const body = await res.text().catch(() => "");
		throw new Error(`HTTP ${res.status}: ${body}`);
	}
	return res.json() as Promise<T>;
}

export async function fetchRawEvents(
	accessToken?: string,
	tenantId?: string | null,
	limit = 50,
	offset = 0,
): Promise<RawEventsResponse> {
	const url = new URL(BASE_URL);
	url.searchParams.set("limit", String(limit));
	url.searchParams.set("offset", String(offset));

	const res = await fetch(url.toString(), {
		headers: authHeaders(accessToken, tenantId ?? undefined),
	});

	return handleResponse<RawEventsResponse>(res);
}
