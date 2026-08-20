import { useAdminMetrics } from "@/hooks/useAdmin";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Loader2, Cpu, HardDrive, Activity, Users, Eye, FileText, Heart, MessageSquare, UserPlus, Gauge } from "lucide-react";
import type { ElementType } from "react";
import type { DayViewCount, HostMetrics } from "@/types/api";

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

function ViewsChart({ byDay }: { byDay: DayViewCount[] }) {
  const max = byDay.reduce((m, d) => Math.max(m, d.views), 0) || 1;
  return (
    <div className="flex h-32 items-end gap-1.5">
      {byDay.map((d) => (
        <div key={d.day} className="group flex flex-1 flex-col items-center gap-1" title={`${d.day}: ${d.views} views`}>
          <div
            className="w-full rounded-t bg-primary/25 transition-colors group-hover:bg-primary/50"
            style={{ height: `${Math.max((d.views / max) * 100, d.views > 0 ? 4 : 1)}%` }}
          />
          <span className="text-[10px] text-muted-foreground">{d.day.slice(5)}</span>
        </div>
      ))}
    </div>
  );
}

export default function MetricsDashboard() {
  const { data, isLoading, isError } = useAdminMetrics();

  if (isLoading) {
    return <div className="flex justify-center py-16"><Loader2 className="h-6 w-6 animate-spin" /></div>;
  }

  const metrics = data?.data;
  if (isError || !metrics) {
    return <p className="py-10 text-center text-muted-foreground">Failed to load metrics.</p>;
  }

  const host: HostMetrics = metrics.host;
  const { app, active, views } = metrics;

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
        <CardHeader>
          <CardTitle className="text-base text-primary">Views — last 14 days</CardTitle>
        </CardHeader>
        <CardContent>
          {views.by_day.length === 0 ? (
            <p className="text-sm text-muted-foreground">No traffic recorded yet.</p>
          ) : (
            <ViewsChart byDay={views.by_day} />
          )}
        </CardContent>
      </Card>
    </div>
  );
}
