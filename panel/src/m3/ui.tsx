// Shared M3 UI primitives (pure CSS + tiny JS, no new dependencies).
import { useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent, ReactNode } from "react";

export function ripple(e: ReactPointerEvent<HTMLElement>): void {
  if (e.currentTarget.querySelector?.(".m3-ink")) return;
  const el = e.currentTarget;
  const r = el.getBoundingClientRect();
  const d = Math.max(r.width, r.height) * 2;
  const ink = document.createElement("span");
  ink.className = "m3-ink";
  ink.style.width = ink.style.height = `${d}px`;
  ink.style.left = `${e.clientX - r.left - d / 2}px`;
  ink.style.top = `${e.clientY - r.top - d / 2}px`;
  el.appendChild(ink);
  ink.addEventListener("animationend", () => ink.remove());
}

export function M3Button(props: {
  variant?: "filled" | "tonal" | "error";
  icon?: string;
  small?: boolean;
  disabled?: boolean;
  style?: CSSProperties;
  onClick?: () => void;
  children?: ReactNode;
}) {
  const { variant = "filled", icon, small, disabled, style, onClick, children } = props;
  return (
    <button
      type="button"
      className={`btn btn-${variant} m3-ripple${small ? " btn-sm" : ""}`}
      style={style}
      disabled={disabled}
      onClick={onClick}
      onPointerDown={disabled ? undefined : ripple}
    >
      {icon && <span className="msr">{icon}</span>}
      {children}
    </button>
  );
}

export function M3IconBtn(props: {
  icon: string;
  title?: string;
  active?: boolean;
  disabled?: boolean;
  onClick?: () => void;
  className?: string;
}) {
  const { icon, title, active, disabled, onClick, className } = props;
  return (
    <button
      type="button"
      className={`icon-btn m3-ripple${active ? " active" : ""}${className ? ` ${className}` : ""}`}
      title={title}
      aria-label={title}
      disabled={disabled}
      onClick={onClick}
      onPointerDown={disabled ? undefined : ripple}
    >
      <span className="msr">{icon}</span>
    </button>
  );
}

export function M3Field(props: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  trailing?: ReactNode;
  placeholder?: string;
  onEnter?: () => void;
}) {
  const { label, value, onChange, trailing, placeholder, onEnter } = props;
  return (
    <label className="field">
      <input
        value={value}
        placeholder={placeholder ?? " "}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onEnter?.();
        }}
      />
      <span className="field-label">{label}</span>
      {trailing}
    </label>
  );
}

export function M3Switch(props: { checked: boolean; onChange: (v: boolean) => void }) {
  const { checked, onChange } = props;
  return (
    <span className="switch">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="track" />
      <span className="thumb" />
    </span>
  );
}

export function M3Select(props: {
  label: string;
  value: string;
  options: string[];
  onChange: (v: string) => void;
}) {
  const { label, value, options, onChange } = props;
  const [open, setOpen] = useState(false);
  return (
    <div className="select">
      <button type="button" className="field m3-ripple" onClick={() => setOpen(!open)}>
        <span className="select-value" style={value ? undefined : { color: "var(--md-on-surface-variant)" }}>
          {value || label}
        </span>
        <span className="msr">arrow_drop_down</span>
      </button>
      {open && (
        <>
          <div className="menu-backdrop" onClick={() => setOpen(false)} />
          <div className="select-menu">
            {options.map((o) => (
              <button
                key={o}
                type="button"
                className={`select-opt${o === value ? " sel" : ""}`}
                onClick={() => {
                  onChange(o);
                  setOpen(false);
                }}
              >
                <span>{o}</span>
                {o === value && <span className="msr">check</span>}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

export function EmbedCard(props: { title: string; children: ReactNode }) {
  const { title, children } = props;
  return (
    <section className="card embed-card">
      <h2 className="card-title">{title}</h2>
      <div className="embed-body">{children}</div>
    </section>
  );
}
