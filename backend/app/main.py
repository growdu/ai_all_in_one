"""AI All-in-One Backend (1.0 scaffold)

最小可运行骨架：后续按 docs/roadmap/05-mvp-tasks.md 增量实现。
"""
from fastapi import FastAPI

app = FastAPI(
    title="AI All-in-One",
    version="0.1.0",
    description="统一大模型入口，详见 ../docs/",
)


@app.get("/health")
async def health():
    return {"status": "ok", "version": "0.1.0"}


# 路由挂载（按 docs/backend/02-provider.md 实施时逐个打开）：
# app.include_router(models_router, prefix="/api/v1")
# app.include_router(chat_router, prefix="/api/v1")
# app.include_router(keys_router, prefix="/api/v1")
# app.include_router(files_router, prefix="/api/v1")
