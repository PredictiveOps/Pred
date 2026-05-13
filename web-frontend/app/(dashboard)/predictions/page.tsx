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
import { fetchPredictions, type Prediction } from "@/lib/predictions-api";

const DEFAULT_LIMIT = 10;

type HumanFeedback = "correct" | "wrong";

type ViewState = {
	loading: boolean;
	error?: string;
	predictions: Prediction[];
	total: number;
};

function formatDate(value: string) {
	try {
		return new Date(value).toLocaleString();
	} catch {
		return value;
	}
}

const STATUS_STYLES: Record<string, string> = {
	normal: "bg-green-100 text-green-700",
	warning: "bg-yellow-100 text-yellow-700",
	critical: "bg-red-100 text-red-700",
};

const REVIEW_STYLES: Record<string, string> = {
	pending_review: "bg-blue-100 text-blue-700",
	reviewed: "bg-green-100 text-green-700",
	archived: "bg-gray-100 text-gray-500",
};

function statusBadge(value: string, styles: Record<string, string>) {
	const cls = styles[value.toLowerCase()] ?? "bg-gray-100 text-gray-700";
	return (
		<span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${cls}`}>
			{value.replace(/_/g, " ").toUpperCase()}
		</span>
	);
}

export default function PredictionsPage() {
	const { data: session, status } = useSession();
	const [state, setState] = useState<ViewState>({
		loading: true,
		predictions: [],
		total: 0,
	});
	const [humanFeedback, setHumanFeedback] = useState<
		Record<string, HumanFeedback>
	>({});
	const [page, setPage] = useState(0);

	function setPredictionFeedback(
		predictionId: string,
		feedback: HumanFeedback,
	) {
		setHumanFeedback((prev) => ({
			...prev,
			[predictionId]: feedback,
		}));
	}

	useEffect(() => {
		if (status === "loading") {
			return;
		}

		const accessToken = session?.accessToken;
		const tenantId = session?.tenantId;

		setState((prev) => ({ ...prev, loading: true, error: undefined }));

		fetchPredictions(accessToken, tenantId, DEFAULT_LIMIT, page * DEFAULT_LIMIT)
			.then((res) => {
				setState({
					loading: false,
					predictions: res.predictions,
					total: res.count,
				});
			})
			.catch((err: Error) => {
				setState({
					loading: false,
					predictions: [],
					total: 0,
					error: err.message,
				});
			});
	}, [page, session?.accessToken, session?.tenantId, status]);

	const rows = useMemo(() => state.predictions, [state.predictions]);
	const totalPages = Math.max(1, Math.ceil(state.total / DEFAULT_LIMIT));

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-semibold text-gray-900">Predictions</h1>
				<p className="text-sm text-gray-500">
					ML model predictions from the bearing anomaly detection pipeline.
				</p>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Prediction Results</CardTitle>
					<CardDescription>
						{state.loading
							? "Loading predictions..."
							: `${rows.length} of ${state.total} predictions`}
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
									<th className="px-3">Asset</th>
									<th className="px-3">Device</th>
									<th className="px-3">Status</th>
									<th className="px-3">Anomaly Score</th>
									<th className="px-3">Review</th>
									<th className="px-3">Model</th>
									<th className="px-3">Human Check</th>
								</tr>
							</thead>
							<tbody>
								{rows.map((prediction) => {
									const feedback = humanFeedback[prediction.prediction_id];

									return (
										<tr
											key={prediction.prediction_id}
											className="rounded-lg bg-white shadow-xs ring-1 ring-gray-200"
										>
											<td className="px-3 py-3 text-gray-700">
												{formatDate(prediction.timestamp)}
											</td>
											<td className="px-3 py-3 font-mono text-xs text-gray-700">
												{prediction.asset_id}
											</td>
											<td className="px-3 py-3 font-mono text-xs text-gray-700">
												{prediction.device_id}
											</td>
											<td className="px-3 py-3">
												{statusBadge(
													prediction.predicted_status,
													STATUS_STYLES,
												)}
											</td>
											<td className="px-3 py-3 text-gray-700">
												{prediction.anomaly_score.toFixed(4)}
											</td>
											<td className="px-3 py-3">
												{statusBadge(prediction.review_status, REVIEW_STYLES)}
											</td>
											<td className="px-3 py-3 text-gray-500">
												{prediction.model_name}{" "}
												<span className="text-gray-400">
													v{prediction.model_version}
												</span>
											</td>
											<td className="px-3 py-3">
												<div className="flex min-w-36 items-center gap-2">
													<Button
														type="button"
														size="xs"
														variant={
															feedback === "correct" ? "default" : "outline"
														}
														className={
															feedback === "correct"
																? "bg-green-600 text-white hover:bg-green-700"
																: "border-green-200 text-green-700 hover:bg-green-50"
														}
														aria-pressed={feedback === "correct"}
														onClick={() =>
															setPredictionFeedback(
																prediction.prediction_id,
																"correct",
															)
														}
													>
														Correct
													</Button>
													<Button
														type="button"
														size="xs"
														variant={
															feedback === "wrong" ? "destructive" : "outline"
														}
														className={
															feedback === "wrong"
																? ""
																: "border-red-200 text-red-700 hover:bg-red-50"
														}
														aria-pressed={feedback === "wrong"}
														onClick={() =>
															setPredictionFeedback(
																prediction.prediction_id,
																"wrong",
															)
														}
													>
														Wrong
													</Button>
												</div>
											</td>
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
