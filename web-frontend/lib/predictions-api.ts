const BASE_URL = `${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8000"}/api/predictions`;

type AuthHeaders = Record<string, string>;

export type Prediction = {
	prediction_id: string;
	tenant_id: string;
	device_id: string;
	asset_id: string;
	model_name: string;
	model_version: string;
	anomaly_score: number;
	predicted_status: string;
	review_status: string;
	reviewed: boolean;
	timestamp: string;
};

export type PredictionsResponse = {
	tenant_id: string;
	count: number;
	predictions: Prediction[];
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

export async function fetchPredictions(
	accessToken?: string,
	tenantId?: string | null,
	limit = 50,
	offset = 0,
): Promise<PredictionsResponse> {
	const url = new URL(BASE_URL);
	url.searchParams.set("limit", String(limit));
	url.searchParams.set("offset", String(offset));

	const res = await fetch(url.toString(), {
		headers: authHeaders(accessToken, tenantId ?? undefined),
	});

	return handleResponse<PredictionsResponse>(res);
}
