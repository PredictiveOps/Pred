"use client";

import { useSession } from "next-auth/react";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { fetchRawEvents, type RawEvent } from "@/lib/events-api";

const DEFAULT_LIMIT = 10;

type ViewState = {
	loading: boolean;
	error?: string;
	events: RawEvent[];
	total: number;
};

function formatDate(value: string | number) {
	try {
		return new Date(value).toLocaleString();
	} catch {
		return value;
	}
}

function formatColumnName(key: string) {
	return key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function isTimestampKey(key: string) {
	return /(_at|_time|_date|timestamp)$/i.test(key);
}

function isStatusKey(key: string) {
	return /status/i.test(key);
}

const STATUS_STYLES: Record<string, string> = {
	normal: "bg-green-100 text-green-700",
	ok: "bg-green-100 text-green-700",
	warning: "bg-yellow-100 text-yellow-700",
	warn: "bg-yellow-100 text-yellow-700",
	critical: "bg-red-100 text-red-700",
	error: "bg-red-100 text-red-700",
	info: "bg-blue-100 text-blue-700",
};

function statusStyle(value: string) {
	return STATUS_STYLES[value.toLowerCase()] ?? "bg-gray-100 text-gray-700";
}

function renderPayloadValue(key: string, value: unknown) {
	if (value === undefined || value === null || value === "-") {
		return <span className="text-gray-400">—</span>;
	}
	console.log(key, value);
	if (
		isTimestampKey(key) &&
		(typeof value === "string" || typeof value === "number")
	) {
		return formatDate(value);
	}
	if (key.toLowerCase() === "mode" && typeof value === "string") {
		return value.charAt(0).toUpperCase().concat(value.slice(1));
	}
	if (isStatusKey(key) && typeof value === "string") {
		return (
			<span
				className={`rounded-full px-2 py-0.5 text-xs font-semibold ${statusStyle(value)}`}
			>
				{value.toUpperCase()}
			</span>
		);
	}
	if (typeof value === "number") {
		return value.toFixed(2);
	}
	if (typeof value === "string") {
		return value;
	}
	return JSON.stringify(value);
}

export default function RawEventsPage() {
	const { data: session, status } = useSession();
	const [state, setState] = useState<ViewState>({
		loading: true,
		events: [],
		total: 0,
	});
	const [page, setPage] = useState(0);

	useEffect(() => {
		if (status === "loading") {
			return;
		}

		const accessToken = session?.accessToken;
		const tenantId = session?.tenantId;

		setState((prev) => ({ ...prev, loading: true, error: undefined }));

		fetchRawEvents(accessToken, tenantId, DEFAULT_LIMIT, page * DEFAULT_LIMIT)
			.then((res) => {
				setState({ loading: false, events: res.events, total: res.count });
			})
			.catch((err: Error) => {
				setState({
					loading: false,
					events: [],
					total: 0,
					error: err.message,
				});
			});
	}, [page, session?.accessToken, session?.tenantId, status]);

	const rows = useMemo(() => state.events, [state.events]);
	const totalPages = Math.max(1, Math.ceil(state.total / DEFAULT_LIMIT));
	const payloadKeys = useMemo(() => {
		const keys = new Set<string>();
		for (const event of rows) {
			for (const key of Object.keys(event.payload ?? {})) {
				if (key !== "tenant_id") keys.add(key);
			}
		}
		return Array.from(keys);
	}, [rows]);

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
							: `${rows.length} of ${state.total} events`}
						.
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
									{payloadKeys.map((key) => (
										<th key={key} className="px-3">
											{formatColumnName(key)}
										</th>
									))}
								</tr>
							</thead>
							<tbody>
								{rows.map((event) => {
									const payload = event.payload ?? {};
									return (
										<tr
											key={event.id}
											className="rounded-lg bg-white shadow-xs ring-1 ring-gray-200"
										>
											<td className="px-3 py-3 text-gray-700">
												{formatDate(event.created_at)}
											</td>
											{payloadKeys.map((key) => (
												<td key={key} className="px-3 py-3 text-gray-700">
													{renderPayloadValue(key, payload[key])}
												</td>
											))}
										</tr>
									);
								})}
							</tbody>
						</table>
					</div>

					<div className="mt-6 flex items-center justify-between gap-3">
						<div className="text-xs text-gray-500">
							Page {page + 1} of {totalPages}
						</div>
						<div className="flex items-center gap-2">
							<Button
								variant="outline"
								size="sm"
								disabled={page === 0 || state.loading}
								onClick={() => setPage((prev) => Math.max(0, prev - 1))}
							>
								Previous
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={page + 1 >= totalPages || state.loading}
								onClick={() => setPage((prev) => prev + 1)}
							>
								Next
							</Button>
						</div>
					</div>
				</CardContent>
			</Card>
		</div>
	);
}
