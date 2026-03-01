import type { LucideIcon } from "lucide-react";
import {
  Braces,
  CalendarClock,
  Cloud,
  Code2,
  Database,
  Fingerprint,
  GitBranch,
  GitFork,
  MessageCircle,
  PlayCircle,
  Shield,
  ShieldAlert,
  ShieldCheck,
  ShieldX,
  Send,
  Server,
  Sigma,
  Timer,
} from "lucide-react";

type NodeTone = "sky" | "emerald" | "amber" | "rose" | "slate" | "cyan";

export type WorkflowNodeVisual = {
  icon: LucideIcon;
  tone: NodeTone;
  badgeClass: string;
  handleClass: string;
};

function buildVisual(icon: LucideIcon, tone: NodeTone): WorkflowNodeVisual {
  const styles: Record<NodeTone, { badgeClass: string; handleClass: string }> = {
    sky: {
      badgeClass: "border-sky-500/35 bg-sky-500/10 text-sky-700 dark:text-sky-300",
      handleClass: "border-sky-500/60 bg-sky-500/20 hover:bg-sky-500/30",
    },
    emerald: {
      badgeClass: "border-emerald-500/35 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
      handleClass: "border-emerald-500/60 bg-emerald-500/20 hover:bg-emerald-500/30",
    },
    amber: {
      badgeClass: "border-amber-500/35 bg-amber-500/10 text-amber-700 dark:text-amber-300",
      handleClass: "border-amber-500/60 bg-amber-500/20 hover:bg-amber-500/30",
    },
    rose: {
      badgeClass: "border-rose-500/35 bg-rose-500/10 text-rose-700 dark:text-rose-300",
      handleClass: "border-rose-500/60 bg-rose-500/20 hover:bg-rose-500/30",
    },
    slate: {
      badgeClass: "border-slate-500/35 bg-slate-500/10 text-slate-700 dark:text-slate-300",
      handleClass: "border-slate-500/60 bg-slate-500/20 hover:bg-slate-500/30",
    },
    cyan: {
      badgeClass: "border-cyan-500/35 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300",
      handleClass: "border-cyan-500/60 bg-cyan-500/20 hover:bg-cyan-500/30",
    },
  };

  return {
    icon,
    tone,
    badgeClass: styles[tone].badgeClass,
    handleClass: styles[tone].handleClass,
  };
}

const CATEGORY_VISUALS: Record<string, WorkflowNodeVisual> = {
  start: buildVisual(PlayCircle, "sky"),
  control: buildVisual(GitBranch, "amber"),
  action: buildVisual(Send, "rose"),
  data: buildVisual(Database, "emerald"),
  compute: buildVisual(Code2, "slate"),
  security: buildVisual(ShieldAlert, "rose"),
};

const NODE_VISUALS: Record<string, WorkflowNodeVisual> = {
  trigger: buildVisual(PlayCircle, "sky"),
  condition: buildVisual(GitBranch, "amber"),
  switch: buildVisual(GitFork, "amber"),
  delay: buildVisual(Timer, "amber"),
  scheduler: buildVisual(CalendarClock, "amber"),
  transform: buildVisual(Braces, "emerald"),
  aggregate: buildVisual(Sigma, "emerald"),
  ioc_extract: buildVisual(Fingerprint, "rose"),
  watchlist_match: buildVisual(ShieldCheck, "rose"),
  mitre_map: buildVisual(Shield, "rose"),
  risk_score: buildVisual(ShieldAlert, "rose"),
  containment_decision: buildVisual(ShieldX, "rose"),
  telegram_send: buildVisual(MessageCircle, "rose"),
  postgres_query: buildVisual(Database, "emerald"),
  redis: buildVisual(Server, "emerald"),
  cassandra_query: buildVisual(Database, "emerald"),
  s3_object: buildVisual(Cloud, "emerald"),
  python_code: buildVisual(Code2, "slate"),
};

const FALLBACK_VISUAL = buildVisual(PlayCircle, "cyan");

export function workflowNodeVisual(nodeType: string, category?: string): WorkflowNodeVisual {
  if (NODE_VISUALS[nodeType]) {
    return NODE_VISUALS[nodeType];
  }
  const normalizedCategory = String(category || "").trim().toLowerCase();
  return CATEGORY_VISUALS[normalizedCategory] || FALLBACK_VISUAL;
}
