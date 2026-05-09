const BASE_URL = `${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8000"}/api/ingest`;

export type DeviceDetails = {
	device_id: number;
	tenant_id: string;
	is_active: boolean;
	created_at: string;
	updated_at: string;
};

function authHeaders(accessToken?: string): HeadersInit {
	return {
		"Content-Type": "application/json",
		...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
	};
}

async function handleResponse<T>(res: Response): Promise<T> {
	if (!res.ok) {
		const body = await res.text().catch(() => "");
		throw new Error(`HTTP ${res.status}: ${body}`);
	}
	return res.json() as Promise<T>;
}

export async function fetchDevicesByTenant(
	tenantId: string,
	accessToken?: string,
): Promise<DeviceDetails[]> {
	const res = await fetch(`${BASE_URL}/devices`, {
		headers: {
			...authHeaders(accessToken),
			"X-Tenant-Id": tenantId,
		},
	});
	return handleResponse<DeviceDetails[]>(res);
}

export async function registerDevice(
	deviceId: number,
	tenantId: string,
	accessToken?: string,
): Promise<void> {
	const res = await fetch(`${BASE_URL}/devices/register`, {
		method: "POST",
		headers: {
			...authHeaders(accessToken),
			"X-Tenant-Id": tenantId,
		},
		body: JSON.stringify({ device_id: deviceId, tenant_id: tenantId }),
	});
	await handleResponse<unknown>(res);
}

export async function updateDeviceStatus(
	deviceId: number,
	tenantId: string,
	isActive: boolean,
	accessToken?: string,
): Promise<void> {
	const res = await fetch(`${BASE_URL}/devices/${deviceId}/status`, {
		method: "PUT",
		headers: {
			...authHeaders(accessToken),
			"X-Tenant-Id": tenantId,
		},
		body: JSON.stringify({ is_active: isActive }),
	});
	await handleResponse<unknown>(res);
}

export async function deleteDevice(
	deviceId: number,
	tenantId: string,
	accessToken?: string,
): Promise<void> {
	const res = await fetch(`${BASE_URL}/devices/${deviceId}`, {
		method: "DELETE",
		headers: {
			...authHeaders(accessToken),
			"X-Tenant-Id": tenantId,
		},
	});
	await handleResponse<unknown>(res);
}
