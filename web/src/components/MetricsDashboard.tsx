import { useState } from "react";
import { useAdminMetrics, useAdminMetricsHistory } from "@/hooks/useAdmin";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Bar, BarChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { Progress } from "@/components/ui/progress";
import { Loader2, Cpu, HardDrive, Activity, Users, Eye, FileText, Heart, MessageSquare, UserPlus, Gauge } from "lucide-react";
import type { ElementType } from "react";
import type { DayViewCount, HistoryRange, HostMetrics, HostSamplePoint, ViewRange } from "@/types/api";

const HOST_RANGES: { value: HistoryRange; label: string }[] = [
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
];

const VIEW_RANGES: { value: ViewRange; label: string }[] = [
  { value: "14d", label: "14 days" },
  { value: "30d", label: "30 days" },
  { value: "90d", label: "90 days" },
];

const usageChartConfig = {
  cpu_percent: { label: "CPU %", color: "var(--chart-1)" },
  mem_percent: { label: "Memory %", color: "var(--chart-2)" },
  disk_percent: { label: "Disk %", color: "var(--chart-3)" },
} satisfies ChartConfig;

const viewsChartConfig = {
  views: { label: "Views", color: "var(--chart-5)" },
} satisfies ChartConfig;

function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDuration(seconds: number): string {
  if (!seconds) return "0s";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function StatCard({ icon: Icon, label, value, sub }: { icon: ElementType; label: string; value: string; sub?: string }) {
  return (
    <Card>
      <CardContent className="flex items-start gap-3">
        <div className="rounded-lg bg-primary/10 p-2 text-primary">
          <Icon className="h-5 w-5" />
        </div>
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-2xl font-bold text-primary truncate">{value}</p>
          {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
        </div>
      </CardContent>
    </Card>
  );
}

function Meter({ label, percent, used, total, icon: Icon }: { label: string; percent: number; used?: number; total?: number; icon: ElementType }) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span className="flex items-center gap-1.5 text-primary">
          <Icon className="h-4 w-4" />
          {label}
        </span>
        <span className="font-medium text-primary">{percent.toFixed(1)}%</span>
      </div>
      <Progress value={Math.min(100, percent)} />
      {used != null && total != null && (
        <p className="text-xs text-muted-foreground">
          {formatBytes(used)} / {formatBytes(total)}
        </p>
      )}
    </div>
  );
}

function RangeTabs<T extends string>({ options, value, onChange }: { options: { value: T; label: string }[]; value: T; onChange: (v: T) => void }) {
  return (
    <Tabs value={value} onValueChange={(v) => onChange(v as T)}>
      <TabsList>
        {options.map((o) => (
          <TabsTrigger key={o.value} value={o.value}>
            {o.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}

function UsageChart({ data, range }: { data: HostSamplePoint[]; range: HistoryRange }) {
  if (data.length === 0) {
    return <p className="py-10 text-center text-sm text-muted-foreground">No host samples recorded yet.</p>;
  }
  const tick = (ts: string) =>
    range === "24h"
      ? new Date(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
      : new Date(ts).toLocaleDateString([], { month: "short", day: "numeric" });
  return (
    <ChartContainer config={usageChartConfig} className="h-64 aspect-auto">
      <LineChart data={data} margin={{ left: 12, right: 12 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="ts" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={tick} />
        <YAxis domain={[0, 100]} tickLine={false} axisLine={false} tickMargin={8} width={36} />
        <ChartTooltip content={<ChartTooltipContent />} />
        <ChartLegend />
        <Line dataKey="cpu_percent" type="monotone" stroke="var(--color-cpu_percent)" strokeWidth={2} dot={false} />
        <Line dataKey="mem_percent" type="monotone" stroke="var(--color-mem_percent)" strokeWidth={2} dot={false} />
        <Line dataKey="disk_percent" type="monotone" stroke="var(--color-disk_percent)" strokeWidth={2} dot={false} />
      </LineChart>
    </ChartContainer>
  );
}

function ViewsChart({ byDay }: { byDay: DayViewCount[] }) {
  if (byDay.length === 0) {
    return <p className="py-10 text-center text-sm text-muted-foreground">No traffic recorded yet.</p>;
  }
  return (
    <ChartContainer config={viewsChartConfig} className="h-64 aspect-auto">
      <BarChart data={byDay} margin={{ left: 12, right: 12 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="day" tickLine={false} axisLine={false} tickMargin={8} tickFormatter={(d: string) => d.slice(5)} />
        <YAxis allowDecimals={false} tickLine={false} axisLine={false} tickMargin={8} width={40} />
        <ChartTooltip cursor={false} content={<ChartTooltipContent />} />
        <Bar dataKey="views" fill="var(--color-views)" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartContainer>
  );
}

export default function MetricsDashboard() {
  const [hostRange, setHostRange] = useState<HistoryRange>("24h");
  const [viewRange, setViewRange] = useState<ViewRange>("14d");
  const { data, isLoading, isError } = useAdminMetrics();
  const history = useAdminMetricsHistory(hostRange, viewRange);

  if (isLoading) {
    return <div className="flex justify-center py-16"><Loader2 className="h-6 w-6 animate-spin" /></div>;
  }

  const metrics = data?.data;
  if (isError || !metrics) {
    return <p className="py-10 text-center text-muted-foreground">Failed to load metrics.</p>;
  }

  const host: HostMetrics = metrics.host;
  const { app, active, views } = metrics;
  const historyData = history.data?.data;

  return (
    <div className="space-y-6">
      <section>
        <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold text-primary">
          <Gauge className="h-5 w-5" /> Host
        </h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <Card>
            <CardHeader><CardTitle className="text-base text-primary">CPU &amp; memory</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <Meter label="CPU" percent={host.cpu_percent} icon={Cpu} />
              <Meter label="Memory" percent={host.mem_percent} used={host.mem_used} total={host.mem_total} icon={Activity} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle className="text-base text-primary">Disk</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <Meter label="Disk usage" percent={host.disk_percent} used={host.disk_used} total={host.disk_total} icon={HardDrive} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle className="text-base text-primary">Load &amp; uptime</CardTitle></CardHeader>
            <CardContent className="space-y-1 text-sm text-primary">
              <p>Load average <span className="font-medium">1m {host.load1.toFixed(2)}</span> · <span className="font-medium">5m {host.load5.toFixed(2)}</span> · <span className="font-medium">15m {host.load15.toFixed(2)}</span></p>
              <p>Uptime <span className="font-medium">{formatDuration(host.uptime_seconds)}</span></p>
            </CardContent>
          </Card>
        </div>
      </section>

      <section>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-primary">
            <Gauge className="h-5 w-5" /> Host usage over time
          </h2>
          <RangeTabs options={HOST_RANGES} value={hostRange} onChange={setHostRange} />
        </div>
        <Card>
          <CardContent className="pt-4">
            {history.isError ? (
              <p className="py-10 text-center text-sm text-muted-foreground">Failed to load host history.</p>
            ) : history.isLoading ? (
              <div className="flex justify-center py-16"><Loader2 className="h-6 w-6 animate-spin" /></div>
            ) : (
              <UsageChart data={historyData?.host ?? []} range={hostRange} />
            )}
          </CardContent>
        </Card>
      </section>

      <section>
        <h2 className="mb-3 text-lg font-semibold text-primary">Platform</h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard icon={Users} label="Users" value={app.users.toLocaleString()} sub={`+${app.signups_24h} in 24h`} />
          <StatCard icon={FileText} label="Posts" value={app.posts.toLocaleString()} />
          <StatCard icon={Heart} label="Likes" value={app.likes.toLocaleString()} />
          <StatCard icon={MessageSquare} label="Messages" value={app.messages.toLocaleString()} />
          <StatCard icon={Users} label="Active today" value={active.dau.toLocaleString()} sub={`${active.wau.toLocaleString()} in 7d`} />
          <StatCard icon={Eye} label="Views" value={app.views_total.toLocaleString()} sub={`${views.requests_per_minute} / min`} />
          <StatCard icon={UserPlus} label="New users / 24h" value={app.signups_24h.toLocaleString()} />
        </div>
      </section>

      <Card>
        <CardHeader className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="text-base text-primary">Views</CardTitle>
          <RangeTabs options={VIEW_RANGES} value={viewRange} onChange={setViewRange} />
        </CardHeader>
        <CardContent className="pt-0">
          {history.isError ? (
            <p className="py-10 text-center text-sm text-muted-foreground">Failed to load view history.</p>
          ) : history.isLoading ? (
            <div className="flex justify-center py-16"><Loader2 className="h-6 w-6 animate-spin" /></div>
          ) : (
            <ViewsChart byDay={historyData?.views ?? []} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}