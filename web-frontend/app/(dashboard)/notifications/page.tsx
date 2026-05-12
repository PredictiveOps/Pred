"use client";

import {
	AlertCircle,
	AlertTriangle,
	Bell,
	CheckCircle2,
	Loader2,
	RefreshCcw,
	XCircle,
} from "lucide-react";
import { useSession } from "next-auth/react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

type Notification = {
	id: number;
	tenant_id: string;
	type: string;
	payload: unknown;
	created_at: string;
};

const BASE_URL = `${process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8000"}/api/notifications`;
console.log({ BASE_URL });

function formatDate(value: string) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return new Intl.DateTimeFormat("en", {
		dateStyle: "medium",
		timeStyle: "short",
	}).format(date);
}

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

function summarizePayload(payload: unknown): string {
	if (!payload || typeof payload !== "object" || Array.isArray(payload))
		return "No structured payload";
	const obj = payload as Record<string, unknown>;
	if (typeof obj.message === "string" && obj.message.trim()) return obj.message;
	if (typeof obj.title === "string" && obj.title.trim()) return obj.title;
	const prob = getFailureProbability(payload);
	if (prob !== null) return `Failure probability ${Math.round(prob * 100)}%`;
	return `${Object.keys(obj).length} payload fields`;
}

export default function NotificationsPage() {
	const { data: session } = useSession();
	const [notifications, setNotifications] = useState<Notification[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [limit, setLimit] = useState(20);

	const tenantId = session?.tenantId ?? "";
	const accessToken = session?.accessToken;

	const loadNotifications = useCallback(async () => {
		if (!tenantId) return;
		setLoading(true);
		setError(null);
		try {
			const res = await fetch(`${BASE_URL}/list?limit=${limit}`, {
				headers: {
					"Content-Type": "application/json",
					"X-Tenant-Id": tenantId,
					...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
				},
				cache: "no-store",
			});
			if (!res.ok) {
				const body = await res.text().catch(() => "");
				throw new Error(`HTTP ${res.status}: ${body}`);
			}
			const data = (await res.json()) as Notification[];
			setNotifications(data);
		} catch (err) {
			setError(
				err instanceof Error
					? err.message
					: "Failed to load notifications. Is the notifications service running?",
			);
			setNotifications([]);
		} finally {
			setLoading(false);
		}
	}, [tenantId, accessToken, limit]);

	useEffect(() => {
		if (session) void loadNotifications();
	}, [session, loadNotifications]);

	const stats = useMemo(() => {
		const emailCount = notifications.filter((n) => n.type === "email").length;
		const pushCount = notifications.filter((n) => n.type === "push").length;
		const highRiskCount = notifications.filter((n) => {
			const p = getFailureProbability(n.payload);
			return p !== null && p >= 0.8;
		}).length;
		return {
			total: notifications.length,
			emailCount,
			pushCount,
			highRiskCount,
		};
	}, [notifications]);

	const statCards = [
		{
			label: "TOTAL ALERTS",
			value: String(stats.total),
			icon: Bell,
			color: "text-blue-600",
		},
		{
			label: "EMAIL ALERTS",
			value: String(stats.emailCount),
			icon: CheckCircle2,
			color: "text-green-600",
		},
		{
			label: "HIGH-RISK ALERTS",
			value: String(stats.highRiskCount),
			icon: AlertCircle,
			color: stats.highRiskCount > 0 ? "text-red-500" : "text-gray-400",
		},
	];

	return (
		<div className="space-y-6">
			<div className="flex items-start justify-between mb-6">
				<div>
					<h1 className="text-3xl font-bold mb-2">Notifications</h1>
					<p className="text-gray-500 text-sm">
						{loading
							? "Loading notifications…"
							: `${stats.total} notification${stats.total === 1 ? "" : "s"}${tenantId ? ` for tenant ${tenantId}` : ""}`}
					</p>
				</div>
				<Button
					type="button"
					onClick={() => void loadNotifications()}
					disabled={loading || !tenantId}
					className="bg-blue-600 hover:bg-blue-700 text-white px-6 font-semibold uppercase text-sm flex items-center gap-2"
				>
					{loading ? (
						<Loader2 className="w-4 h-4 animate-spin" />
					) : (
						<RefreshCcw className="w-4 h-4" />
					)}
					REFRESH
				</Button>
			</div>

			<div className="grid grid-cols-3 gap-4">
				{statCards.map((stat) => {
					const Icon = stat.icon;
					return (
						<Card key={stat.label} className="bg-white border-gray-200 p-6">
							<div className="flex justify-between items-start mb-4">
								<div className="text-xs text-gray-500 font-semibold uppercase">
									{stat.label}
								</div>
								<Icon className={`w-4 h-4 ${stat.color}`} />
							</div>
							<div className={`text-2xl font-bold ${stat.color}`}>
								{loading ? <Skeleton className="h-7 w-12" /> : stat.value}
							</div>
						</Card>
					);
				})}
			</div>

			<Card className="bg-white border-gray-200 p-6">
				<div className="flex justify-between items-center mb-6">
					<h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
						Notification Log
					</h2>
					<div className="flex items-center gap-2 text-sm text-gray-500">
						<label
							htmlFor="limit-select"
							className="text-xs font-semibold uppercase text-gray-400"
						>
							Limit
						</label>
						<select
							id="limit-select"
							value={limit}
							onChange={(e) => setLimit(Number(e.target.value))}
							className="border border-gray-200 rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
						>
							{[10, 20, 50, 100].map((n) => (
								<option key={n} value={n}>
									{n}
								</option>
							))}
						</select>
					</div>
				</div>

				{error && (
					<div className="flex items-center gap-2 text-red-500 text-sm mb-4">
						<XCircle className="w-4 h-4 flex-shrink-0" />
						{error}
					</div>
				)}

				{!tenantId && !loading && (
					<p className="text-gray-400 text-sm text-center py-8">
						No tenant ID found in session. Ensure Keycloak is configured with a{" "}
						<code className="text-xs bg-gray-100 px-1 rounded">tenant_id</code>{" "}
						claim.
					</p>
				)}

				{tenantId && (
					<div className="overflow-x-auto">
						<table className="w-full">
							<thead>
								<tr className="border-b border-gray-200">
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										ID
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Type
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Summary
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Risk
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Created At
									</th>
								</tr>
							</thead>
							<tbody>
								{loading &&
									["s1", "s2", "s3", "s4", "s5"].map((sk) => (
										<tr key={sk} className="border-b border-gray-100">
											{["c1", "c2", "c3", "c4", "c5", "c6"].map((col) => (
												<td key={col} className="py-4 px-4">
													<Skeleton className="h-4 w-full" />
												</td>
											))}
										</tr>
									))}

								{!loading && notifications.length === 0 && (
									<tr>
										<td
											colSpan={6}
											className="py-10 text-center text-gray-400 text-sm"
										>
											No notifications found for this tenant.
										</td>
									</tr>
								)}

								{!loading &&
									notifications.map((n) => {
										const probability = getFailureProbability(n.payload);
										const isHighRisk =
											probability !== null && probability >= 0.8;

										return (
											<tr
												key={n.id}
												className="border-b border-gray-100 hover:bg-gray-50 transition-colors"
											>
												<td className="py-4 px-4">
													<span className="text-sm font-semibold text-gray-800">
														#{n.id}
													</span>
												</td>
												<td className="py-4 px-4">
													<span
														className={`text-xs font-semibold px-2 py-1 rounded uppercase ${
															n.type === "email"
																? "bg-blue-50 text-blue-600"
																: n.type === "push"
																	? "bg-green-50 text-green-600"
																	: "bg-gray-100 text-gray-600"
														}`}
													>
														{n.type}
													</span>
												</td>
												<td className="py-4 px-4 text-sm text-gray-600 max-w-xs truncate">
													{summarizePayload(n.payload)}
												</td>
												<td className="py-4 px-4">
													{probability !== null ? (
														<div className="flex items-center gap-1">
															<AlertTriangle
																className={`w-3 h-3 ${isHighRisk ? "text-red-500" : "text-yellow-500"}`}
															/>
															<span
																className={`text-xs font-semibold ${isHighRisk ? "text-red-500" : "text-yellow-600"}`}
															>
																{Math.round(probability * 100)}%
															</span>
														</div>
													) : (
														<span className="text-xs text-gray-300">—</span>
													)}
												</td>
												<td className="py-4 px-4 text-sm text-gray-600">
													{formatDate(n.created_at)}
												</td>
											</tr>
										);
									})}
							</tbody>
						</table>
					</div>
				)}
			</Card>
		</div>
	);
}
