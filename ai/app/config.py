from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    PORT: int = 8000
    INTERNAL_AI_SECRET: str = "lynk-ai-internal-secret-key-2026"
    DATABASE_URL: str = (
        "postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable"
    )
    VLLM_URL: str = "http://localhost:8000/v1"
    EMBEDDING_MODEL_NAME: str = "all-MiniLM-L6-v2"
    EMBEDDING_DIMENSION: int = 384
    LOG_LEVEL: str = "INFO"

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )


@lru_cache
def get_settings() -> Settings:
    return Settings()
