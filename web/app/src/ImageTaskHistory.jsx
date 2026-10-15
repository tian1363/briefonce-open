import { useEffect, useState } from "react";
import { listProviderImageTasks } from "./api.js";
import DownloadableImage from "./DownloadableImage.jsx";
export default function ImageTaskHistory() {
  const [rows, setRows] = useState([]),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    async function refresh() {
      try {
        const items = await listProviderImageTasks();
        if (active) setRows(items);
      } catch (e) {
        if (active) setError(e.message);
      }
    }
    refresh();
    const timer = setInterval(refresh, 5000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, []);
  return (
    <details className="image-task-history">
      <summary>图片任务与可恢复结果（{rows.length}）</summary>
      {error && <p role="alert">{error}</p>}
      {rows.length ? (
        <div className="purchase-package-grid">
          {rows.map((r) => (
            <article key={r.id}>
              <strong>{r.model}</strong>
              <p>
                {{
                  pending: "待提交",
                  submitting: "提交中",
                  queued: "排队中",
                  in_progress: "生成中",
                  saving: "正在保存",
                  uncertain: "待管理员核对",
                  completed: "已完成",
                  failed: "失败",
                }[r.status] || r.status}{" "}
                · {r.points} 积分
              </p>
              {r.status === "completed" && (
                <DownloadableImage
                  src={`/api/image-provider-jobs/${r.id}/image`}
                  alt="生成结果"
                  filename={`${r.model}-${r.id}.png`}
                />
              )}
              <small>
                {r.error || new Date(r.created_at).toLocaleString("zh-CN")}
              </small>
            </article>
          ))}
        </div>
      ) : (
        <p>暂无新供应商图片任务。</p>
      )}
    </details>
  );
}
