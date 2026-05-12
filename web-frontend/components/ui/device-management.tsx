"use client";

import {
	AlertCircle,
	CheckCircle2,
	Loader2,
	Plus,
	Trash2,
	XCircle,
} from "lucide-react";
import { useSession } from "next-auth/react";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
	type DeviceDetails,
	deleteDevice,
	fetchDevicesByTenant,
	registerDevice,
	updateDeviceStatus,
} from "@/lib/ingestion-api";

export function DeviceManagement() {
	const { data: session } = useSession();
	const [devices, setDevices] = useState<DeviceDetails[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [showRegisterForm, setShowRegisterForm] = useState(false);
	const [newDeviceId, setNewDeviceId] = useState("");
	const [registering, setRegistering] = useState(false);
	const [registerError, setRegisterError] = useState<string | null>(null);
	const [actionInProgress, setActionInProgress] = useState<number | null>(null);

	const tenantId = session?.tenantId || "";
	const accessToken = session?.accessToken;

	const loadDevices = useCallback(async () => {
		if (!tenantId) return;
		setLoading(true);
		setError(null);
		try {
			const data = await fetchDevicesByTenant(tenantId, accessToken);
			setDevices(data);
		} catch {
			setError("Failed to load devices. Is the ingestion service running?");
		} finally {
			setLoading(false);
		}
	}, [tenantId, accessToken]);

	useEffect(() => {
		if (session) loadDevices();
	}, [session, loadDevices]);

	async function handleRegister(e: React.FormEvent) {
		e.preventDefault();
		const deviceId = Number.parseInt(newDeviceId, 10);
		if (!deviceId || !tenantId) return;
		setRegistering(true);
		setRegisterError(null);
		try {
			await registerDevice(deviceId, tenantId, accessToken);
			setNewDeviceId("");
			setShowRegisterForm(false);
			await loadDevices();
		} catch {
			setRegisterError("Registration failed. Device ID may already exist.");
		} finally {
			setRegistering(false);
		}
	}

	async function handleToggleStatus(device: DeviceDetails) {
		setActionInProgress(device.device_id);
		try {
			await updateDeviceStatus(
				device.device_id,
				tenantId,
				!device.is_active,
				accessToken,
			);
			setDevices((prev) =>
				prev.map((d) =>
					d.device_id === device.device_id
						? { ...d, is_active: !d.is_active }
						: d,
				),
			);
		} catch {
			// silently fail — user can retry
		} finally {
			setActionInProgress(null);
		}
	}

	async function handleDelete(device: DeviceDetails) {
		setActionInProgress(device.device_id);
		try {
			await deleteDevice(device.device_id, tenantId, accessToken);
			setDevices((prev) =>
				prev.filter((d) => d.device_id !== device.device_id),
			);
		} catch {
			// silently fail — user can retry
		} finally {
			setActionInProgress(null);
		}
	}

	const totalDevices = devices.length;
	const activeDevices = devices.filter((d) => d.is_active).length;
	const offlineDevices = devices.filter((d) => !d.is_active).length;

	const stats = [
		{
			label: "TOTAL DEVICES",
			value: String(totalDevices),
			icon: CheckCircle2,
			color: "text-blue-600",
		},
		{
			label: "ACTIVE DEVICES",
			value: String(activeDevices),
			icon: CheckCircle2,
			color: "text-green-600",
		},
		{
			label: "OFFLINE DEVICES",
			value: String(offlineDevices),
			icon: AlertCircle,
			color: offlineDevices > 0 ? "text-red-500" : "text-gray-400",
		},
	];

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-start justify-between mb-6">
				<div>
					<h1 className="text-3xl font-bold mb-2">Device Management</h1>
					<p className="text-gray-500 text-sm">
						{loading
							? "Loading devices…"
							: `${totalDevices} device${totalDevices === 1 ? "" : "s"} registered${tenantId ? ` for tenant ${tenantId}` : ""}`}
					</p>
				</div>
				<Button
					type="button"
					onClick={() => {
						setShowRegisterForm((v) => !v);
						setRegisterError(null);
					}}
					className="bg-blue-600 hover:bg-blue-700 text-white px-6 font-semibold uppercase text-sm flex items-center gap-2"
				>
					<Plus className="w-4 h-4" />
					ADD NEW DEVICE
				</Button>
			</div>

			{/* Register Form */}
			{showRegisterForm && (
				<Card className="bg-white border-gray-200 p-4">
					<form onSubmit={handleRegister} className="flex items-end gap-4">
						<div className="flex flex-col gap-1">
							<label
								htmlFor="device-id"
								className="text-xs font-semibold text-gray-500 uppercase"
							>
								Device ID
							</label>
							<input
								id="device-id"
								type="number"
								min="1"
								required
								value={newDeviceId}
								onChange={(e) => setNewDeviceId(e.target.value)}
								placeholder="e.g. 101"
								className="border border-gray-300 rounded px-3 py-2 text-sm w-40 focus:outline-none focus:ring-2 focus:ring-blue-500"
							/>
						</div>
						<div className="flex flex-col gap-1">
							<span className="text-xs font-semibold text-gray-500 uppercase">
								Tenant ID
							</span>
							<span className="border border-gray-200 bg-gray-50 rounded px-3 py-2 text-sm text-gray-500 w-40">
								{tenantId ?? "—"}
							</span>
						</div>
						<Button
							type="submit"
							disabled={registering || !tenantId}
							className="bg-blue-600 hover:bg-blue-700 text-white px-4 font-semibold text-sm"
						>
							{registering ? (
								<Loader2 className="w-4 h-4 animate-spin" />
							) : (
								"Register"
							)}
						</Button>
						<Button
							type="button"
							onClick={() => setShowRegisterForm(false)}
							className="bg-gray-100 hover:bg-gray-200 text-gray-700 px-4 text-sm"
						>
							Cancel
						</Button>
					</form>
					{registerError && (
						<p className="text-red-500 text-xs mt-2">{registerError}</p>
					)}
				</Card>
			)}

			{/* Stats Cards */}
			<div className="grid grid-cols-3 gap-4">
				{stats.map((stat) => {
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

			{/* Device Table */}
			<Card className="bg-white border-gray-200 p-6">
				<div className="flex justify-between items-center mb-6">
					<h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wider">
						Device Registry
					</h2>
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
										Device ID
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Status
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Registered At
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-left py-3 px-4">
										Last Updated
									</th>
									<th className="text-xs text-gray-500 font-semibold uppercase text-center py-3 px-4">
										Actions
									</th>
								</tr>
							</thead>
							<tbody>
								{loading &&
									["s1", "s2", "s3", "s4"].map((sk) => (
										<tr key={sk} className="border-b border-gray-100">
											{["c1", "c2", "c3", "c4", "c5", "c6"].map((col) => (
												<td key={col} className="py-4 px-4">
													<Skeleton className="h-4 w-full" />
												</td>
											))}
										</tr>
									))}

								{!loading && devices.length === 0 && (
									<tr>
										<td
											colSpan={5}
											className="py-10 text-center text-gray-400 text-sm"
										>
											No devices registered yet. Use "Add New Device" to
											register one.
										</td>
									</tr>
								)}

								{!loading &&
									devices.map((device) => {
										const busy = actionInProgress === device.device_id;
										return (
											<tr
												key={device.device_id}
												className="border-b border-gray-100 hover:bg-gray-50 transition-colors"
											>
												<td className="py-4 px-4">
													<span className="text-sm font-semibold text-gray-800">
														#{device.device_id}
													</span>
												</td>
												<td className="py-4 px-4">
													<div className="flex items-center gap-2">
														<div
															className={`w-2 h-2 rounded-full ${device.is_active ? "bg-green-500" : "bg-red-500"}`}
														/>
														<span
															className={`text-xs font-semibold ${device.is_active ? "text-green-600" : "text-red-500"}`}
														>
															{device.is_active ? "ACTIVE" : "OFFLINE"}
														</span>
													</div>
												</td>
												<td className="py-4 px-4 text-sm text-gray-600">
													{new Date(device.created_at).toLocaleDateString()}
												</td>
												<td className="py-4 px-4 text-sm text-gray-600">
													{new Date(device.updated_at).toLocaleDateString()}
												</td>
												<td className="py-4 px-4">
													<div className="flex items-center justify-center gap-2">
														<button
															type="button"
															disabled={busy}
															onClick={() => handleToggleStatus(device)}
															className={`text-xs font-semibold px-2 py-1 rounded transition-colors ${
																device.is_active
																	? "bg-yellow-50 text-yellow-600 hover:bg-yellow-100"
																	: "bg-green-50 text-green-600 hover:bg-green-100"
															} disabled:opacity-50`}
														>
															{busy ? (
																<Loader2 className="w-3 h-3 animate-spin" />
															) : device.is_active ? (
																"Deactivate"
															) : (
																"Activate"
															)}
														</button>
														<button
															type="button"
															disabled={busy}
															onClick={() => handleDelete(device)}
															className="p-1 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors disabled:opacity-50"
														>
															{busy ? (
																<Loader2 className="w-4 h-4 animate-spin" />
															) : (
																<Trash2 className="w-4 h-4" />
															)}
														</button>
													</div>
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
