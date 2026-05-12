"use client";

import {
	AlertCircle,
	AlertTriangle,
	Bell,
	CheckCircle2,
	Loader2,
	RefreshCcw,
	Zap,
} from "lucide-react";
import { useSession } from "next-auth/react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchPredictions, type Prediction } from "@/lib/predictions-api";
import { fetchRawEvents } from "@/lib/events-api";

const NOTIFICATIONS_BASE = `${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8000"}/api/notifications`;

type Notification = {
	id: number;
	tenant_id: string;
	type: string;
	payload: unknown;
	created_at: string;
};

function getFailureProbability(payload: unknown): number | null {
	if (!payload || typeof payload !== "object" || Array.isArray(payload))
		return null;
	const p = (payload as Record<string, unknown>).failure_probability;
	if (typeof p === "number") return p;
	if (typeof p === "string") {
		const n = Number.parseFloat(p);
		return Number.isNaN(n) ? null : n;
	}
	return null;
}

function formatDate(value: string) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return new Intl.DateTimeFormat("en", {
		dateStyle: "medium",
		timeStyle: "short",
	}).format(date);
}

const STATUS_BADGE: Record<string, string> = {
	normal: "bg-green-100 text-green-700",
	warning: "bg-yellow-100 text-yellow-700",
	critical: "bg-red-100 text-red-700",
};

export default function DashboardPage() {
	const { data: session, status } = useSession();

	const [predictions, setPredictions] = useState<Prediction[]>([]);
	const [notifications, setNotifications] = useState<Notification[]>([]);
	const [eventsTotal, setEventsTotal] = useState<number | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	const accessToken = session?.accessToken;
	const tenantId = session?.tenantId ?? "";

	const load = useCallback(async () => {
		if (status === "loading") return;
		setLoading(true);
		setError(null);
		try {
			const [predsRes, eventsRes, notifRes] = await Promise.allSettled([
				fetchPredictions(accessToken, tenantId, 50, 0),
				fetchRawEvents(accessToken, tenantId, 1, 0),
				fetch(`${NOTIFICATIONS_BASE}/list?limit=5`, {
					headers: {
						"Content-Type": "application/json",
						...(tenantId ? { "X-Tenant-Id": tenantId } : {}),
						...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
					},
					cache: "no-store",
				}),
			]);

			if (predsRes.status === "fulfilled") {
				setPredictions(predsRes.value.predictions);
			}
			if (eventsRes.status === "fulfilled") {
				setEventsTotal(eventsRes.value.count);
			}
			if (notifRes.status === "fulfilled" && notifRes.value.ok) {
				const data = (await notifRes.value.json()) as Notification[];
				setNotifications(data);
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to load data.");
		} finally {
			setLoading(false);
		}
	}, [accessToken, tenantId, status]);

	useEffect(() => {
		void load();
	}, [load]);

	const stats = useMemo(() => {
		const normal = predictions.filter(
			(p) => p.predicted_status === "normal",
		).length;
		const warning = predictions.filter(
			(p) => p.predicted_status === "warning",
		).length;
		const critical = predictions.filter(
			(p) => p.predicted_status === "critical",
		).length;
		const highRisk = notifications.filter((n) => {
			const p = getFailureProbability(n.payload);
			return p !== null && p >= 0.8;
		}).length;
		return { normal, warning, critical, highRisk, total: predictions.length };
	}, [predictions, notifications]);

	const recentPredictions = useMemo(
		() => predictions.slice(0, 5),
		[predictions],
	);

	const statCards = [
		{
			label: "NORMAL",
			value: stats.normal,
			icon: CheckCircle2,
			color: "text-green-600",
			bar: "bg-green-500",
		},
		{
			label: "WARNING",
			value: stats.warning,
			icon: AlertTriangle,
			color: "text-yellow-500",
			bar: "bg-yellow-500",
		},
		{
			label: "CRITICAL",
			value: stats.critical,
			icon: AlertCircle,
			color: "text-red-500",
			bar: "bg-red-500",
		},
		{
			label: "HIGH-RISK ALERTS",
			value: stats.highRisk,
			icon: Bell,
			color: stats.highRisk > 0 ? "text-red-500" : "text-gray-400",
			bar: stats.highRisk > 0 ? "bg-red-500" : "bg-gray-200",
		},
	];

	return (
		<div className="space-y-6">
			<div className="flex items-start justify-between">
				<div>
					<h1 className="text-2xl font-semibold text-gray-900">Dashboard</h1>
					<p className="text-sm text-gray-500 mt-1">
						{loading
							? "Loading system overview…"
							: `${stats.total} predictions · ${eventsTotal ?? "—"} events ingested`}
					</p>
				</div>
				<Button
					type="button"
					onClick={() => void load()}
					disabled={loading || status === "loading"}
					className="bg-blue-600 hover:bg-blue-700 text-white px-5 font-semibold uppercase text-sm flex items-center gap-2"
				>
					{loading ? (
						<Loader2 className="w-4 h-4 animate-spin" />
					) : (
						<RefreshCcw className="w-4 h-4" />
					)}
					REFRESH
				</Button>
			</div>

			{error && (
				<div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 flex items-center gap-2">
					<AlertCircle className="w-4 h-4 flex-shrink-0" />
					{error}
				</div>
			)}

			<div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
				{statCards.map((s) => {
					const Icon = s.icon;
					return (
						<Card key={s.label} className="bg-white border-gray-200">
							<CardContent className="p-5">
								<div className="flex justify-between items-start mb-3">
									<span className="text-xs text-gray-500 font-semibold uppercase">
										{s.label}
									</span>
									<Icon className={`w-4 h-4 ${s.color}`} />
								</div>
								<div className={`text-3xl font-bold ${s.color}`}>
									{loading ? <Skeleton className="h-8 w-10" /> : s.value}
								</div>
								<div className={`mt-3 h-1 ${s.bar} rounded`} />
							</CardContent>
						</Card>
					);
				})}
			</div>

			<div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<Card className="bg-white border-gray-200">
					<CardHeader className="pb-3">
						<CardTitle className="text-sm font-semibold text-gray-700 uppercase tracking-wider flex items-center gap-2">
							<Zap className="w-4 h-4 text-blue-500" />
							Recent Predictions
						</CardTitle>
					</CardHeader>
					<CardContent>
						<div className="overflow-x-auto">
							<table className="w-full text-sm">
								<thead>
									<tr className="border-b border-gray-100">
										<th className="text-xs text-gray-400 font-semibold uppercase text-left py-2 px-3">
											Asset
										</th>
										<th className="text-xs text-gray-400 font-semibold uppercase text-left py-2 px-3">
											Status
										</th>
										<th className="text-xs text-gray-400 font-semibold uppercase text-left py-2 px-3">
											Score
										</th>
										<th className="text-xs text-gray-400 font-semibold uppercase text-left py-2 px-3">
											Time
										</th>
									</tr>
								</thead>
								<tbody>
									{loading &&
										[1, 2, 3, 4, 5].map((i) => (
											<tr key={i} className="border-b border-gray-50">
												{[1, 2, 3, 4].map((j) => (
													<td key={j} className="py-3 px-3">
														<Skeleton className="h-4 w-full" />
													</td>
												))}
											</tr>
										))}
									{!loading && recentPredictions.length === 0 && (
										<tr>
											<td
												colSpan={4}
												className="py-8 text-center text-sm text-gray-400"
											>
												No predictions available.
											</td>
										</tr>
									)}
									{!loading &&
										recentPredictions.map((p) => {
											const badgeCls =
												STATUS_BADGE[p.predicted_status.toLowerCase()] ??
												"bg-gray-100 text-gray-700";
											return (
												<tr
													key={p.prediction_id}
													className="border-b border-gray-50 hover:bg-gray-50 transition-colors"
												>
													<td className="py-3 px-3 font-mono text-xs text-gray-700">
														{p.asset_id}
													</td>
													<td className="py-3 px-3">
														<span
															className={`rounded-full px-2 py-0.5 text-xs font-semibold ${badgeCls}`}
														>
															{p.predicted_status.toUpperCase()}
														</span>
													</td>
													<td className="py-3 px-3 text-gray-600">
														{p.anomaly_score.toFixed(3)}
													</td>
													<td className="py-3 px-3 text-gray-500 text-xs">
														{formatDate(p.timestamp)}
													</td>
												</tr>
											);
										})}
								</tbody>
							</table>
						</div>
					</CardContent>
				</Card>

				<Card className="bg-white border-gray-200">
					<CardHeader className="pb-3">
						<CardTitle className="text-sm font-semibold text-gray-700 uppercase tracking-wider flex items-center gap-2">
							<Bell className="w-4 h-4 text-yellow-500" />
							Recent Notifications
						</CardTitle>
					</CardHeader>
					<CardContent>
						<div className="space-y-2">
							{loading &&
								[1, 2, 3, 4, 5].map((i) => (
									<div
										key={i}
										className="flex gap-3 items-start p-3 rounded-lg border border-gray-100"
									>
										<Skeleton className="w-4 h-4 rounded-full flex-shrink-0 mt-0.5" />
										<div className="flex-1 space-y-1.5">
											<Skeleton className="h-3 w-24" />
											<Skeleton className="h-3 w-full" />
										</div>
									</div>
								))}
							{!loading && notifications.length === 0 && (
								<p className="py-8 text-center text-sm text-gray-400">
									No recent notifications.
								</p>
							)}
							{!loading &&
								notifications.map((n) => {
									const prob = getFailureProbability(n.payload);
									const isHighRisk = prob !== null && prob >= 0.8;
									const Icon = isHighRisk ? AlertCircle : AlertTriangle;
									const iconCls = isHighRisk
										? "text-red-500"
										: n.type === "email"
											? "text-blue-500"
											: "text-yellow-500";
									const labelCls = isHighRisk ? "text-red-500" : "text-gray-500";
									const label = isHighRisk
										? "HIGH RISK"
										: n.type.toUpperCase();
									return (
										<div
											key={n.id}
											className="flex gap-3 items-start p-3 rounded-lg border border-gray-100 hover:bg-gray-50 transition-colors"
										>
											<Icon
												className={`w-4 h-4 flex-shrink-0 mt-0.5 ${iconCls}`}
											/>
											<div className="flex-1 min-w-0">
												<div
													className={`text-xs font-semibold ${labelCls} uppercase`}
												>
													{label}
													{prob !== null && (
														<span className="ml-2 font-normal text-gray-400">
															{Math.round(prob * 100)}% failure probability
														</span>
													)}
												</div>
												<div className="text-xs text-gray-500 mt-0.5">
													{formatDate(n.created_at)}
												</div>
											</div>
										</div>
									);
								})}
						</div>
					</CardContent>
				</Card>
			</div>
		</div>
	);
}
