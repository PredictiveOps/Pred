"use client";

import { useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { fetchRawEvents, type RawEvent } from "@/lib/events-api";

const DEFAULT_LIMIT = 20;

type ViewState = {
	loading: boolean;
	error?: string;
	events: RawEvent[];
};

function formatDate(value: string) {
	try {
		return new Date(value).toLocaleString();
	} catch {
		return value;
	}
}

function renderPayloadValue(value: unknown) {
	if (typeof value === "number") {
		return value.toFixed(2);
	}
	if (typeof value === "string") {
		return value;
	}
	return JSON.stringify(value);
}

function payloadEntries(payload: Record<string, unknown>) {
	return Object.entries(payload).filter(([key]) => key !== "tenant_id");
}

export default function RawEventsPage() {
	const { data: session, status } = useSession();
	const [state, setState] = useState<ViewState>({
		loading: true,
		events: [],
	});

	useEffect(() => {
		if (status === "loading") {
			return;
		}

		const accessToken = session?.accessToken;
		const tenantId = session?.tenantId;

		setState({ loading: true, events: [] });

		fetchRawEvents(accessToken, tenantId, DEFAULT_LIMIT, 0)
			.then((res) => {
				setState({ loading: false, events: res.events });
			})
			.catch((err: Error) => {
				setState({
					loading: false,
					events: [],
					error: err.message,
				});
			});
	}, [session?.accessToken, session?.tenantId, status]);

	const rows = useMemo(() => state.events, [state.events]);

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-semibold text-gray-900">Raw Events</h1>
				<p className="text-sm text-gray-500">
					Latest raw sensor events from the event-processing service.
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Event Stream</CardTitle>
					<CardDescription>
						{state.loading
							? "Loading events..."
							: `${rows.length} events`}.
					</CardDescription>
				</CardHeader>
				<CardContent>
					{state.error ? (
						<div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
							{state.error}
						</div>
					) : null}

					<div className="overflow-x-auto">
						<table className="min-w-full border-separate border-spacing-y-3 text-sm">
							<thead className="text-left text-xs uppercase text-gray-400">
								<tr>
									<th className="px-3">Time</th>
									<th className="px-3">Device</th>
									<th className="px-3">Status</th>
									<th className="px-3">V RMS</th>
									<th className="px-3">Temp (C)</th>
									<th className="px-3">Peaks (Hz)</th>
									<th className="px-3">Payload</th>
								</tr>
							</thead>
							<tbody>
								{rows.map((event) => {
									const payload = event.payload ?? {};
									const entries = payloadEntries(payload);
									return (
										<tr
											key={event.id}
											className="rounded-lg bg-white shadow-xs ring-1 ring-gray-200"
										>
											<td className="px-3 py-3 text-gray-700">
												{formatDate(event.created_at)}
											</td>
											<td className="px-3 py-3 font-medium text-gray-900">
												{String(payload.device_id ?? "-")}
											</td>
											<td className="px-3 py-3">
												<span className="rounded-full bg-gray-100 px-2 py-1 text-xs font-semibold text-gray-700">
													{String(payload.status ?? "-")}
												</span>
											</td>
											<td className="px-3 py-3 text-gray-700">
												{payload.v_rms ?? "-"}
											</td>
											<td className="px-3 py-3 text-gray-700">
												{payload.temp_c ?? "-"}
											</td>
											<td className="px-3 py-3 text-gray-700">
												{[payload.peak_hz_1, payload.peak_hz_2, payload.peak_hz_3]
													.filter((value) => value !== undefined)
													.join(", ") || "-"}
											</td>
											<td className="px-3 py-3">
												<div className="grid gap-1 rounded-md border border-gray-200 bg-gray-50 p-2 text-xs text-gray-600">
													{entries.length === 0 ? (
														<span className="text-gray-400">No payload data</span>
													) : (
														entries.map(([key, value]) => (
															<div key={key} className="flex items-center justify-between gap-3">
																<span className="font-medium text-gray-700">{key}</span>
																<span className="text-gray-500">{renderPayloadValue(value)}</span>
															</div>
														))
													)}
												</div>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>
				</CardContent>
			</Card>
		</div>
	);
}
