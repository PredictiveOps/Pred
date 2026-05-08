"use client";

import {
	AlertTriangle,
	Bell,
	Clock3,
	Database,
	RefreshCcw,
	Search,
	Sparkles,
	Wifi,
	WifiOff,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";

type NotificationRecord = {
	id: number;
	tenant_id: string;
	type: string;
	payload: unknown;
	created_at: string;
};

const API_BASE_URL =
	process.env.NEXT_PUBLIC_NOTIFICATIONS_API_URL ?? "http://localhost:8080";
const DEFAULT_TENANT_ID = "factory1";

function normalizePayload(payload: unknown) {
	if (payload && typeof payload === "object" && !Array.isArray(payload)) {
		return payload as Record<string, unknown>;
	}

	if (typeof payload === "string") {
		try {
			const parsed = JSON.parse(payload);
			if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
				return parsed as Record<string, unknown>;
			}
		} catch {
			return null;
		}
	}

	return null;
}

function getFailureProbability(payload: unknown) {
	const normalized = normalizePayload(payload);
	if (!normalized) {
		return null;
	}

	const probability = normalized.failure_probability;
	if (typeof probability === "number") {
		return probability;
	}
	if (typeof probability === "string") {
		const parsed = Number.parseFloat(probability);
		return Number.isNaN(parsed) ? null : parsed;
	}

	return null;
}

function summarizePayload(payload: unknown) {
	const normalized = normalizePayload(payload);
	if (!normalized) {
		return "No structured payload";
	}

	const message = normalized.message;
	if (typeof message === "string" && message.trim()) {
		return message;
	}

	const title = normalized.title;
	if (typeof title === "string" && title.trim()) {
		return title;
	}

	const probability = getFailureProbability(normalized);
	if (probability !== null) {
		return `Failure probability ${Math.round(probability * 100)}%`;
	}

	return `${Object.keys(normalized).length} payload fields`;
}

function formatDate(value: string) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return value;
	}

	return new Intl.DateTimeFormat("en", {
		dateStyle: "medium",
		timeStyle: "short",
	}).format(date);
}

export default function Home() {
	const [tenantId, setTenantId] = useState(DEFAULT_TENANT_ID);
	const [limit, setLimit] = useState("10");
	const [notifications, setNotifications] = useState<NotificationRecord[]>([]);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [lastUpdated, setLastUpdated] = useState<string | null>(null);
	const [wsConnected, setWsConnected] = useState(false);
	const wsRef = useRef<WebSocket | null>(null);
	const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

	const loadNotifications = useCallback(
		async (nextTenantId: string, nextLimit: string) => {
			const trimmedTenantId = nextTenantId.trim();
			const normalizedLimit = Number.parseInt(nextLimit, 10);

			if (!trimmedTenantId) {
				setError("Enter a tenant ID to load notifications.");
				setNotifications([]);
				return;
			}

			setLoading(true);
			setError(null);

			try {
				const query = new URLSearchParams({
					tenant_id: trimmedTenantId,
					limit:
						Number.isFinite(normalizedLimit) && normalizedLimit > 0
							? String(normalizedLimit)
							: "10",
				});

				const response = await fetch(
					`${API_BASE_URL}/notifications?${query.toString()}`,
					{
						cache: "no-store",
					},
				);

				if (!response.ok) {
					const body = await response.text();
					throw new Error(
						body || `Request failed with status ${response.status}`,
					);
				}

				const data = (await response.json()) as NotificationRecord[];
				setNotifications(data);
				setLastUpdated(new Date().toISOString());
			} catch (fetchError) {
				setError(
					fetchError instanceof Error
						? fetchError.message
						: "Failed to load notifications.",
				);
				setNotifications([]);
			} finally {
				setLoading(false);
			}
		},
		[],
	);

	// WebSocket connection for real-time notifications
	useEffect(() => {
		if (!tenantId.trim()) {
			setWsConnected(false);
			if (wsRef.current) {
				wsRef.current.close();
				wsRef.current = null;
			}
			return;
		}

		const connectWebSocket = () => {
			try {
				const wsBaseUrl = API_BASE_URL.replace(/^http/, "ws");
				const ws = new WebSocket(
					`${wsBaseUrl}/ws?tenant_id=${encodeURIComponent(tenantId)}`,
				);

				ws.onopen = () => {
					console.log("WebSocket connected for tenant:", tenantId);
					setWsConnected(true);
					if (reconnectTimeoutRef.current) {
						clearTimeout(reconnectTimeoutRef.current);
						reconnectTimeoutRef.current = null;
					}
				};

				ws.onmessage = (event) => {
					try {
						const message = JSON.parse(event.data);
						if (message.type === "new_notification") {
							// Prepend new notification to the list
							const newNotif: NotificationRecord = {
								id: Math.floor(Math.random() * 1000000),
								tenant_id: tenantId,
								type: "push", // Default type; could be parsed from message
								payload: message.data,
								created_at: new Date().toISOString(),
							};
							setNotifications((prev) => [newNotif, ...prev]);
							setLastUpdated(new Date().toISOString());
						}
					} catch (parseError) {
						console.error("Failed to parse WebSocket message:", parseError);
					}
				};

				ws.onerror = (event) => {
					console.error("WebSocket error:", event);
					setWsConnected(false);
				};

				ws.onclose = () => {
					console.log("WebSocket closed");
					setWsConnected(false);
					// Attempt to reconnect after 3 seconds
					if (reconnectTimeoutRef.current) {
						clearTimeout(reconnectTimeoutRef.current);
					}
					reconnectTimeoutRef.current = setTimeout(connectWebSocket, 3000);
				};

				wsRef.current = ws;
			} catch (err) {
				console.error("Failed to create WebSocket:", err);
				setWsConnected(false);
			}
		};

		connectWebSocket();

		return () => {
			if (reconnectTimeoutRef.current) {
				clearTimeout(reconnectTimeoutRef.current);
				reconnectTimeoutRef.current = null;
			}
			if (wsRef.current) {
				wsRef.current.close();
				wsRef.current = null;
			}
		};
	}, [tenantId]);

	useEffect(() => {
		void loadNotifications(DEFAULT_TENANT_ID, "10");
	}, [loadNotifications]);

	const stats = useMemo(() => {
		const emailCount = notifications.filter(
			(item) => item.type === "email",
		).length;
		const pushCount = notifications.filter(
			(item) => item.type === "push",
		).length;
		const highRiskCount = notifications.filter((item) => {
			const probability = getFailureProbability(item.payload);
			return probability !== null && probability >= 0.8;
		}).length;

		return {
			total: notifications.length,
			emailCount,
			pushCount,
			highRiskCount,
		};
	}, [notifications]);

	return (
		<div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(34,197,94,0.14),_transparent_30%),linear-gradient(180deg,_#08111f_0%,_#050816_100%)] text-white">
			<main className="mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-8">
				<section className="overflow-hidden rounded-[2rem] border border-white/10 bg-white/5 p-6 shadow-2xl shadow-black/20 backdrop-blur-xl sm:p-8">
					<div className="grid gap-8 lg:grid-cols-[1.4fr_0.9fr] lg:items-center">
						<div className="space-y-6">
							<div className="inline-flex items-center gap-2 rounded-full border border-emerald-400/30 bg-emerald-400/10 px-3 py-1 text-sm text-emerald-200">
								<Sparkles className="size-4" />
								Notification dashboard
							</div>
							<div className="space-y-3">
								<h1 className="max-w-2xl text-4xl font-semibold tracking-tight text-white sm:text-5xl">
									Live alerts from the notification service.
								</h1>
								<p className="max-w-2xl text-base leading-7 text-slate-300 sm:text-lg">
									Load saved notifications from the backend, inspect payloads,
									and verify that the email + database flow is working end to
									end.
								</p>
							</div>
							<div className="grid gap-3 sm:grid-cols-3">
								<Card className="border-white/10 bg-white/5 text-white">
									<CardContent className="flex items-center gap-3 px-5 py-4">
										<div className="rounded-2xl bg-cyan-400/15 p-3 text-cyan-200">
											<Bell className="size-5" />
										</div>
										<div>
											<p className="text-2xl font-semibold">{stats.total}</p>
											<p className="text-sm text-slate-300">Total alerts</p>
										</div>
									</CardContent>
								</Card>
								<Card className="border-white/10 bg-white/5 text-white">
									<CardContent className="flex items-center gap-3 px-5 py-4">
										<div className="rounded-2xl bg-emerald-400/15 p-3 text-emerald-200">
											<Database className="size-5" />
										</div>
										<div>
											<p className="text-2xl font-semibold">
												{stats.emailCount}
											</p>
											<p className="text-sm text-slate-300">Email alerts</p>
										</div>
									</CardContent>
								</Card>
								<Card className="border-white/10 bg-white/5 text-white">
									<CardContent className="flex items-center gap-3 px-5 py-4">
										<div className="rounded-2xl bg-amber-400/15 p-3 text-amber-200">
											<AlertTriangle className="size-5" />
										</div>
										<div>
											<p className="text-2xl font-semibold">
												{stats.highRiskCount}
											</p>
											<p className="text-sm text-slate-300">High-risk alerts</p>
										</div>
									</CardContent>
								</Card>
							</div>
						</div>

						<Card className="border-white/10 bg-slate-950/70 text-white shadow-2xl shadow-black/30">
							<CardHeader>
								<CardTitle className="text-xl text-white">
									Load alerts
								</CardTitle>
								<CardDescription className="text-slate-400">
									Use a tenant ID and limit to query the notifications API.
								</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<form
									className="grid gap-3"
									onSubmit={(event) => {
										event.preventDefault();
										void loadNotifications(tenantId, limit);
									}}
								>
									<label className="grid gap-2 text-sm text-slate-300">
										Tenant ID
										<div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 focus-within:border-emerald-400/60">
											<Search className="size-4 text-slate-400" />
											<input
												className="w-full bg-transparent text-white outline-none placeholder:text-slate-500"
												name="tenantId"
												placeholder="factory1"
												value={tenantId}
												onChange={(event) => setTenantId(event.target.value)}
											/>
										</div>
									</label>

									<label className="grid gap-2 text-sm text-slate-300">
										Limit
										<input
											className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-white outline-none placeholder:text-slate-500 focus:border-emerald-400/60"
											name="limit"
											type="number"
											min="1"
											max="100"
											value={limit}
											onChange={(event) => setLimit(event.target.value)}
										/>
									</label>

									<div className="flex flex-wrap gap-3 pt-2">
										<Button
											type="submit"
											className="bg-emerald-400 text-slate-950 hover:bg-emerald-300"
										>
											<RefreshCcw className="size-4" />
											{loading ? "Loading..." : "Refresh alerts"}
										</Button>
										<Button
											type="button"
											variant="outline"
											className="border-white/15 bg-white/5 text-white hover:bg-white/10"
											onClick={() => {
												setTenantId(DEFAULT_TENANT_ID);
												setLimit("10");
												void loadNotifications(DEFAULT_TENANT_ID, "10");
											}}
										>
											Reset to sample
										</Button>
									</div>
								</form>

								{error ? (
									<div className="rounded-2xl border border-red-400/30 bg-red-400/10 px-4 py-3 text-sm text-red-200">
										{error}
									</div>
								) : null}

								<div className="flex items-center justify-between rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-slate-300">
									<div className="flex items-center gap-2">
										<Clock3 className="size-4" />
										<span>
											{lastUpdated
												? `Last updated ${formatDate(lastUpdated)}`
												: "No data loaded yet"}
										</span>
									</div>
									<div className="flex items-center gap-4">
										<span className="text-slate-500">API: {API_BASE_URL}</span>
										<div className="flex items-center gap-1">
											{wsConnected ? (
												<>
													<Wifi className="size-4 text-emerald-400" />
													<span className="text-emerald-300">Live</span>
												</>
											) : (
												<>
													<WifiOff className="size-4 text-slate-500" />
													<span className="text-slate-400">Offline</span>
												</>
											)}
										</div>
									</div>
								</div>
							</CardContent>
						</Card>
					</div>
				</section>

				<section className="grid gap-4">
					{notifications.length === 0 && !loading ? (
						<Card className="border-white/10 bg-white/5 text-white">
							<CardContent className="px-6 py-10 text-center text-slate-300">
								No alerts found for this tenant. Try a different tenant ID or
								send a new Kafka event.
							</CardContent>
						</Card>
					) : null}

					<div className="grid gap-4 lg:grid-cols-2">
						{notifications.map((notification) => {
							const probability = getFailureProbability(notification.payload);
							const isEmail = notification.type === "email";
							const isPush = notification.type === "push";

							return (
								<Card
									key={notification.id}
									className="border-white/10 bg-white/5 text-white transition-transform duration-200 hover:-translate-y-1 hover:border-emerald-400/30 hover:bg-white/8"
								>
									<CardHeader>
										<div className="flex items-center justify-between gap-3">
											<div>
												<CardTitle className="text-lg text-white">
													Notification #{notification.id}
												</CardTitle>
												<CardDescription className="text-slate-400">
													Tenant {notification.tenant_id} •{" "}
													{formatDate(notification.created_at)}
												</CardDescription>
											</div>
											<span
												className={`rounded-full px-3 py-1 text-xs font-medium uppercase tracking-wide ${
													isEmail
														? "bg-cyan-400/15 text-cyan-200"
														: isPush
															? "bg-emerald-400/15 text-emerald-200"
															: "bg-white/10 text-slate-200"
												}`}
											>
												{notification.type}
											</span>
										</div>
									</CardHeader>
									<CardContent className="space-y-4">
										<p className="text-sm leading-6 text-slate-200">
											{summarizePayload(notification.payload)}
										</p>

										<div className="rounded-2xl border border-white/10 bg-slate-950/40 p-4 text-sm text-slate-300">
											<pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-6">
												{JSON.stringify(notification.payload, null, 2)}
											</pre>
										</div>

										{probability !== null ? (
											<div className="inline-flex items-center gap-2 rounded-full bg-amber-400/10 px-3 py-1 text-sm text-amber-200">
												<AlertTriangle className="size-4" />
												Failure probability: {Math.round(probability * 100)}%
											</div>
										) : null}
									</CardContent>
								</Card>
							);
						})}
					</div>
				</section>
			</main>
		</div>
	);
}
