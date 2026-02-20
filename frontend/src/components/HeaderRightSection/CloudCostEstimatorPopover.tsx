import { useMemo, useState } from 'react';
import { useQuery } from 'react-query';
import { Alert, Button, Skeleton, Typography } from 'antd';
import { getQueryRangeV5 } from 'api/v5/queryRange/getQueryRange';
import { ENTITY_VERSION_V5 } from 'constants/app';
import { REACT_QUERY_KEY } from 'constants/reactQueryKeys';
import { ArrowUpRight, CircleHelp, FileText, Gauge, Orbit } from 'lucide-react';
import { QueryRangePayloadV5 } from 'types/api/v5/queryRange';

const KB_PER_GB = 1_048_576;
const SAMPLES_PER_MILLION = 1_000_000;
const THIRTY_DAYS_SECONDS = 30 * 24 * 60 * 60;
const TRACE_PRICE_PER_GB = 0.3;
const LOG_PRICE_PER_GB = 0.3;
const METRIC_PRICE_PER_MILLION = 0.1;

const MIGRATION_DOC_URL =
	'https://signoz.io/docs/migration/migrate-from-signoz-self-host-to-signoz-cloud/';

const formatCurrency = (value: number): string =>
	new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: 'USD',
		maximumFractionDigits: 2,
	}).format(Number.isFinite(value) ? value : 0);

const formatCompact = (value: number): string =>
	new Intl.NumberFormat('en-US', {
		notation: 'compact',
		maximumFractionDigits: 1,
	}).format(value || 0);

const getAggregations = (
	metricName?: string,
): QueryRangePayloadV5['compositeQuery']['queries'][number]['spec']['aggregations'] =>
	metricName
		? [
				{
					metricName,
					temporality: '',
					timeAggregation: 'increase',
					spaceAggregation: 'sum',
					reduceTo: 'avg',
				},
		  ]
		: [{ expression: 'count()' }];

const buildScalarCountPayload = ({
	start,
	end,
	signal,
	source,
	metricName,
}: {
	start: number;
	end: number;
	signal: 'metrics' | 'logs' | 'traces';
	source?: 'meter';
	metricName?: string;
}): QueryRangePayloadV5 => {
	return {
		schemaVersion: 'v1',
		start,
		end,
		requestType: 'scalar',
		compositeQuery: {
			queries: [
				{
					type: 'builder_query',
					spec: {
						name: 'A',
						signal,
						...(source ? { source } : {}),
						stepInterval: null,
						disabled: false,
						filter: { expression: '' },
						legend: 'count',
						aggregations: getAggregations(metricName),
					},
				},
			],
		},
		formatOptions: {
			formatTableResultForUI: false,
			fillGaps: false,
		},
		variables: {},
	};
};

const extractScalarValue = (
	response: Awaited<ReturnType<typeof getQueryRangeV5>>,
): number => {
	const rows = response?.data?.data?.data?.results?.[0]?.data;
	if (!rows || !Array.isArray(rows) || rows.length === 0) {
		return 0;
	}

	return rows.reduce((acc, row) => acc + Number(row?.[0] || 0), 0);
};

// eslint-disable-next-line sonarjs/cognitive-complexity
function CloudCostEstimatorPopover(): JSX.Element {
	const [timeWindow] = useState(() => {
		const nowInSeconds = Math.floor(Date.now() / 1000);
		return {
			nowInNanoseconds: nowInSeconds * 1_000_000_000,
			monthlyStartInNanoseconds:
				(nowInSeconds - THIRTY_DAYS_SECONDS) * 1_000_000_000,
		};
	});

	const { nowInNanoseconds, monthlyStartInNanoseconds } = timeWindow;

	const {
		data: meterSpanCount,
		isLoading: isMeterSpanLoading,
		isError: isMeterSpanError,
	} = useQuery(
		[
			REACT_QUERY_KEY.GET_QUERY_RANGE,
			'cloud-cost-meter-span',
			monthlyStartInNanoseconds,
		],
		async () => {
			const payload = buildScalarCountPayload({
				start: monthlyStartInNanoseconds,
				end: nowInNanoseconds,
				signal: 'metrics',
				source: 'meter',
				metricName: 'signoz.meter.span.count',
			});
			const response = await getQueryRangeV5(payload, ENTITY_VERSION_V5);
			return extractScalarValue(response);
		},
		{ staleTime: 5 * 60 * 1000, retry: 1 },
	);

	const {
		data: meterLogCount,
		isLoading: isMeterLogLoading,
		isError: isMeterLogError,
	} = useQuery(
		[
			REACT_QUERY_KEY.GET_QUERY_RANGE,
			'cloud-cost-meter-log',
			monthlyStartInNanoseconds,
		],
		async () => {
			const payload = buildScalarCountPayload({
				start: monthlyStartInNanoseconds,
				end: nowInNanoseconds,
				signal: 'metrics',
				source: 'meter',
				metricName: 'signoz.meter.log.count',
			});
			const response = await getQueryRangeV5(payload, ENTITY_VERSION_V5);
			return extractScalarValue(response);
		},
		{ staleTime: 5 * 60 * 1000, retry: 1 },
	);

	const {
		data: meterMetricCount,
		isLoading: isMeterMetricLoading,
		isError: isMeterMetricError,
	} = useQuery(
		[
			REACT_QUERY_KEY.GET_QUERY_RANGE,
			'cloud-cost-meter-metric-datapoint',
			monthlyStartInNanoseconds,
		],
		async () => {
			const payload = buildScalarCountPayload({
				start: monthlyStartInNanoseconds,
				end: nowInNanoseconds,
				signal: 'metrics',
				source: 'meter',
				metricName: 'signoz.meter.metric.datapoint.count',
			});
			const response = await getQueryRangeV5(payload, ENTITY_VERSION_V5);
			return extractScalarValue(response);
		},
		{ staleTime: 5 * 60 * 1000, retry: 1 },
	);

	const {
		data: fallbackTraceCount,
		isLoading: isFallbackTraceLoading,
	} = useQuery(
		[
			REACT_QUERY_KEY.GET_QUERY_RANGE,
			'cloud-cost-fallback-traces-count',
			monthlyStartInNanoseconds,
		],
		async () => {
			const payload = buildScalarCountPayload({
				start: monthlyStartInNanoseconds,
				end: nowInNanoseconds,
				signal: 'traces',
			});
			const response = await getQueryRangeV5(payload, ENTITY_VERSION_V5);
			return extractScalarValue(response);
		},
		{ staleTime: 5 * 60 * 1000, retry: 1 },
	);

	const { data: fallbackLogCount, isLoading: isFallbackLogLoading } = useQuery(
		[
			REACT_QUERY_KEY.GET_QUERY_RANGE,
			'cloud-cost-fallback-logs-count',
			monthlyStartInNanoseconds,
		],
		async () => {
			const payload = buildScalarCountPayload({
				start: monthlyStartInNanoseconds,
				end: nowInNanoseconds,
				signal: 'logs',
			});
			const response = await getQueryRangeV5(payload, ENTITY_VERSION_V5);
			return extractScalarValue(response);
		},
		{ staleTime: 5 * 60 * 1000, retry: 1 },
	);

	const spanCount =
		(meterSpanCount || 0) > 0 ? meterSpanCount || 0 : fallbackTraceCount || 0;
	const logCount =
		(meterLogCount || 0) > 0 ? meterLogCount || 0 : fallbackLogCount || 0;
	const metricSampleCount = meterMetricCount || 0;

	const isUsageLoading =
		isMeterSpanLoading ||
		isMeterLogLoading ||
		isMeterMetricLoading ||
		isFallbackTraceLoading ||
		isFallbackLogLoading;

	const isUsageError = isMeterSpanError || isMeterLogError || isMeterMetricError;

	const totalCloudCost = useMemo(() => {
		const traceGb = (spanCount * 2) / KB_PER_GB;
		const logGb = logCount / KB_PER_GB;
		const metricMillions = metricSampleCount / SAMPLES_PER_MILLION;

		return (
			traceGb * TRACE_PRICE_PER_GB +
			logGb * LOG_PRICE_PER_GB +
			metricMillions * METRIC_PRICE_PER_MILLION
		);
	}, [spanCount, logCount, metricSampleCount]);

	const logGbs = useMemo(() => logCount / KB_PER_GB, [logCount]);

	return (
		<div className="cloud-cost-estimator-modal">
			<div className="cloud-cost-header">
				<div className="cloud-cost-header-title">
					<Typography.Text className="cloud-cost-estimator-title">
						Estimated Cloud Cost
					</Typography.Text>
					<CircleHelp size={14} className="cloud-cost-help-icon" />
				</div>
				<Typography.Text className="cloud-cost-estimator-subtitle">
					One-click estimate from last 30 days usage.
				</Typography.Text>
			</div>

			{isUsageLoading ? (
				<Skeleton active paragraph={{ rows: 2 }} title={false} />
			) : (
				<>
					<div className="cloud-cost-section-title">
						<span>Usage Breakdown</span>
						<div className="cloud-cost-section-line" />
					</div>
					<div className="cloud-cost-usage-grid">
						<div className="cloud-cost-usage-card">
							<Typography.Text className="cloud-cost-usage-card-label">
								Spans
							</Typography.Text>
							<Typography.Text className="cloud-cost-usage-card-value">
								{formatCompact(spanCount)}
							</Typography.Text>
						</div>
						<div className="cloud-cost-usage-card">
							<Typography.Text className="cloud-cost-usage-card-label">
								Logs
							</Typography.Text>
							<Typography.Text className="cloud-cost-usage-card-value">
								{logGbs.toFixed(2)}GB
							</Typography.Text>
						</div>
						<div className="cloud-cost-usage-card">
							<Typography.Text className="cloud-cost-usage-card-label">
								Samples
							</Typography.Text>
							<Typography.Text className="cloud-cost-usage-card-value">
								{formatCompact(metricSampleCount)}
							</Typography.Text>
						</div>
					</div>
				</>
			)}

			{isUsageError && (
				<Alert
					type="warning"
					showIcon
					message="Unable to auto-fetch meter usage. Using available fallback data."
				/>
			)}

			<div className="cloud-cost-section-title">
				<span>Cost Rates</span>
				<div className="cloud-cost-section-line" />
			</div>

			<div className="cloud-cost-rates-list">
				<div className="cloud-cost-rate-row">
					<div className="cloud-cost-rate-name">
						<Orbit size={14} />
						<span>Traces</span>
					</div>
					<Typography.Text className="cloud-cost-rate-line">
						${TRACE_PRICE_PER_GB.toFixed(2)} / GB
					</Typography.Text>
				</div>
				<div className="cloud-cost-rate-row">
					<div className="cloud-cost-rate-name">
						<FileText size={14} />
						<span>Logs</span>
					</div>
					<Typography.Text className="cloud-cost-rate-line">
						${LOG_PRICE_PER_GB.toFixed(2)} / GB
					</Typography.Text>
				</div>
				<div className="cloud-cost-rate-row">
					<div className="cloud-cost-rate-name">
						<Gauge size={14} />
						<span>Metrics</span>
					</div>
					<Typography.Text className="cloud-cost-rate-line">
						${METRIC_PRICE_PER_MILLION.toFixed(2)} / 1M samples
					</Typography.Text>
				</div>
			</div>

			<div className="cloud-cost-total-card">
				<Typography.Text className="cloud-cost-total-caption">
					Total Estimated Cost
				</Typography.Text>
				<div className="cloud-cost-total-row">
					<Typography.Text className="cloud-cost-total-value">
						{formatCurrency(totalCloudCost)}
					</Typography.Text>
					<Typography.Text className="cloud-cost-total-suffix">
						/month
					</Typography.Text>
				</div>
			</div>

			<Button
				type="primary"
				href={MIGRATION_DOC_URL}
				target="_blank"
				rel="noreferrer"
				className="cloud-cost-migration-btn"
			>
				<span className="cloud-cost-migration-btn-content">
					<span>Migrate to SigNoz Cloud</span>
					<ArrowUpRight size={14} />
				</span>
			</Button>

			<Typography.Text className="cloud-cost-footnote">
				Calculated based on average ingestion over 30 days
			</Typography.Text>
		</div>
	);
}

export default CloudCostEstimatorPopover;
