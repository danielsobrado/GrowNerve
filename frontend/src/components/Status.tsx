import type { ReactNode } from "react";

export function Status({ tone = "neutral", children }: { tone?: "ok" | "warning" | "critical" | "info" | "neutral" | "simulated"; children: ReactNode }) {
  return <span className={`gn-status gn-status-${tone}`}><span className="gn-status-dot" />{children}</span>;
}

export function Card({ title, subtitle, action, children, className = "" }: { title?: string; subtitle?: string; action?: ReactNode; children: ReactNode; className?: string }) {
  return <section className={`gn-card ${className}`}>
    {(title || action) && <header className="gn-card-head"><div><h2>{title}</h2>{subtitle && <p>{subtitle}</p>}</div>{action}</header>}
    <div className="gn-card-body">{children}</div>
  </section>;
}

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow: string; title: string; description: string; actions?: ReactNode }) {
  return <header className="gn-page-head"><div><p className="gn-eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>{actions && <div className="gn-page-actions">{actions}</div>}</header>;
}

export const formatDateTime = (value?: string) => value ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "—";
export const formatRelative = (value?: string) => {
  if (!value) return "unknown";
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
};
