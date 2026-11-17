// 旧视图承载屏：more_vert 菜单进入，包装 antd App 上下文以供旧视图的 message 使用。
import { App as AntApp } from "antd";
import type { ReactElement } from "react";
import { ApprovalsView } from "../../approvals/ApprovalsView";
import { TasksView } from "../../tasks/TasksView";
import { EventsView } from "../../events/EventsView";
import { UsageView } from "../../usage/UsageView";
import { WorkflowsView } from "../../workflows/WorkflowsView";
import { IntakeView } from "../../intake/IntakeView";
import { KitesurfView } from "../../kitesurf/KitesurfView";

export type LegacyKind =
  | "approvals"
  | "tasks"
  | "events"
  | "usage"
  | "workflows"
  | "intake"
  | "kitesurf";

export const LEGACY_ITEMS: { kind: LegacyKind; icon: string; label: string }[] = [
  { kind: "approvals", icon: "fact_check", label: "审批中心" },
  { kind: "tasks", icon: "task_alt", label: "任务看板" },
  { kind: "events", icon: "stream", label: "事件流" },
  { kind: "usage", icon: "bar_chart", label: "统计看板" },
  { kind: "workflows", icon: "account_tree", label: "协作链" },
  { kind: "intake", icon: "app_registration", label: "接入管理" },
  { kind: "kitesurf", icon: "language", label: "无头浏览器" },
];

export function legacyLabel(kind: LegacyKind): string {
  return LEGACY_ITEMS.find((i) => i.kind === kind)?.label ?? "旧视图";
}

const MAP: Record<LegacyKind, () => ReactElement> = {
  approvals: ApprovalsView,
  tasks: TasksView,
  events: EventsView,
  usage: UsageView,
  workflows: WorkflowsView,
  intake: IntakeView,
  kitesurf: KitesurfView,
};

export function LegacyScreen(props: { kind: LegacyKind }) {
  const View = MAP[props.kind];
  return (
    <AntApp>
      <div className="legacy-wrap">
        <View />
      </div>
    </AntApp>
  );
}
