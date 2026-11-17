// 屏幕2 协作：720dp 填充卡内嵌现有 DifyView。
import { DifyView } from "../../dify/DifyView";
import { EmbedCard } from "../ui";

export function CollabScreen() {
  return (
    <EmbedCard title="Dify">
      <DifyView />
    </EmbedCard>
  );
}
