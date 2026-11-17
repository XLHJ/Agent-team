// 屏幕6 ApexForge：复用现有 ApexForgeView（iframe 嵌入 /apexforge-app/）。
import { ApexForgeView } from "../../apexforge/ApexForgeView";
import { EmbedCard } from "../ui";

export function ForgeScreen() {
  return (
    <EmbedCard title="ApexForge">
      <ApexForgeView />
    </EmbedCard>
  );
}
