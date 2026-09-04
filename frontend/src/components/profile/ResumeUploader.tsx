"use client";

import React, { useState, useRef, useEffect } from "react";
import {
  FileText,
  UploadCloud,
  Download,
  ShieldAlert,
  CheckCircle2,
  AlertCircle,
  Loader2,
  RefreshCw,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { uploadResume, getMyResumeUrl, ApiClientError } from "@/lib/api";
import { formatFileSize, formatJobDate } from "@/lib/formatters";
import type { ResumeUploadResponse } from "@/types/api";

const MAX_FILE_SIZE_BYTES = 5 * 1024 * 1024; // 5MB

export interface ResumeUploaderProps {
  resumeKey?: string | null;
  resumeFilename?: string | null;
  resumeByteSize?: number;
  updatedAt?: string;
  isVerified?: boolean;
  onUploadSuccess?: (uploaded: ResumeUploadResponse) => void;
}

export function ResumeUploader({
  resumeKey: propResumeKey,
  resumeFilename: propResumeFilename,
  resumeByteSize: propResumeByteSize,
  updatedAt: propUpdatedAt,
  isVerified: propIsVerified,
  onUploadSuccess,
}: ResumeUploaderProps) {
  const { isVerified: authIsVerified } = useAuth();
  const isVerified = propIsVerified !== undefined ? propIsVerified : authIsVerified;

  // Local state for active resume
  const [currentResume, setCurrentResume] = useState<{
    key?: string | null;
    filename?: string | null;
    byteSize?: number;
    updatedAt?: string;
  }>({
    key: propResumeKey,
    filename: propResumeFilename,
    byteSize: propResumeByteSize,
    updatedAt: propUpdatedAt,
  });

  // Sync props if parent updates
  useEffect(() => {
    setCurrentResume({
      key: propResumeKey,
      filename: propResumeFilename,
      byteSize: propResumeByteSize,
      updatedAt: propUpdatedAt,
    });
  }, [propResumeKey, propResumeFilename, propResumeByteSize, propUpdatedAt]);

  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [showUploaderOverride, setShowUploaderOverride] = useState(false);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const hasResume = Boolean(currentResume.key || currentResume.filename);

  const validateFile = (file: File): string | null => {
    if (file.size > MAX_FILE_SIZE_BYTES) {
      return "File size exceeds 5MB limit. Please upload a smaller PDF or DOCX file.";
    }
    if (file.size <= 0) {
      return "The selected file is empty. Please select a valid document.";
    }

    const lowerName = file.name.toLowerCase();
    const isPdf = lowerName.endsWith(".pdf") || file.type === "application/pdf";
    const isDocx =
      lowerName.endsWith(".docx") ||
      file.type ===
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

    if (!isPdf && !isDocx) {
      return "Invalid file format. Only PDF (.pdf) and Word (.docx) documents are accepted.";
    }

    return null;
  };

  const handleFileUpload = async (file: File) => {
    setErrorMessage(null);
    setSuccessMessage(null);

    // Strict Institutional Email Verification Gate
    if (!isVerified) {
      setErrorMessage(
        "Institutional .edu verification required. You must verify your university email before uploading your resume."
      );
      return;
    }

    // Client-side file validation
    const validationError = validateFile(file);
    if (validationError) {
      setErrorMessage(validationError);
      return;
    }

    setIsUploading(true);
    try {
      const response = await uploadResume(file);
      setCurrentResume({
        key: response.resume_key,
        filename: response.filename,
        byteSize: response.byte_size,
        updatedAt: new Date().toISOString(),
      });
      setShowUploaderOverride(false);
      setSuccessMessage(response.message || "Resume uploaded successfully!");
      onUploadSuccess?.(response);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        if (err.isEmailNotVerified) {
          setErrorMessage(
            "Institutional .edu email verification required before uploading a resume."
          );
        } else if (err.code === "FILE_TOO_LARGE") {
          setErrorMessage("Resume file exceeds the 5MB size limit.");
        } else if (err.code === "INVALID_FILE_TYPE") {
          setErrorMessage("Resume must be a valid PDF or DOCX file.");
        } else {
          setErrorMessage(err.message || "Failed to upload resume.");
        }
      } else {
        setErrorMessage("An unexpected error occurred while uploading your resume.");
      }
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFileUpload(file);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    if (!isVerified || isUploading) return;
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (!isVerified || isUploading) return;

    const file = e.dataTransfer.files?.[0];
    if (file) {
      handleFileUpload(file);
    }
  };

  const handleDownloadResume = async () => {
    setIsDownloading(true);
    setErrorMessage(null);
    try {
      const res = await getMyResumeUrl();
      const targetUrl = res.download_url || res.url;
      if (targetUrl) {
        window.open(targetUrl, "_blank", "noopener,noreferrer");
      } else {
        setErrorMessage("Presigned download link was empty. Please try again.");
      }
    } catch (err: unknown) {
      if (err instanceof ApiClientError && err.isNotFound) {
        setErrorMessage("Resume not found in storage. Please upload a new one.");
      } else {
        setErrorMessage("Failed to generate secure download link. Please try again.");
      }
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Institutional Email Warning Banner */}
      {!isVerified && (
        <div className="flex items-start gap-3 rounded-[10px] border border-amber-200 bg-amber-50 p-4 text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
          <ShieldAlert className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
          <div className="text-sm">
            <h4 className="font-semibold">Institutional .edu Verification Required</h4>
            <p className="text-xs text-amber-700 dark:text-amber-300 mt-0.5 leading-relaxed">
              Resume uploads are gated for verified university students. Please verify your
              institutional email address via your university login to enable resume storage and job applications.
            </p>
          </div>
        </div>
      )}

      {/* Success Notification */}
      {successMessage && (
        <div className="flex items-center gap-2.5 rounded-[10px] border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300">
          <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>{successMessage}</span>
        </div>
      )}

      {/* Error Alert */}
      {errorMessage && (
        <div className="flex items-center gap-2.5 rounded-[10px] border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Hidden File Input */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        className="hidden"
        onChange={handleFileChange}
        disabled={!isVerified || isUploading}
      />

      {/* State 1: Resume is currently uploaded and not in override upload mode */}
      {hasResume && !showUploaderOverride ? (
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-start gap-3.5">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
                <FileText className="h-6 w-6" />
              </div>
              <div className="min-w-0">
                <h4 className="font-semibold text-slate-900 dark:text-white truncate max-w-sm">
                  {currentResume.filename || "Student_Resume.pdf"}
                </h4>
                <div className="flex flex-wrap items-center gap-2 mt-1 text-xs text-slate-500 dark:text-slate-400">
                  <span>{formatFileSize(currentResume.byteSize)}</span>
                  <span>•</span>
                  <span>
                    Uploaded {formatJobDate(currentResume.updatedAt)}
                  </span>
                  <span>•</span>
                  <span className="text-emerald-600 dark:text-emerald-400 font-medium">
                    Verified MinIO S3 Object
                  </span>
                </div>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={handleDownloadResume}
                disabled={isDownloading}
                className="inline-flex items-center gap-1.5 rounded-[10px] border border-slate-200 bg-white px-3.5 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 hover:text-primary disabled:opacity-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                title="Fetches 15-minute presigned download link"
              >
                {isDownloading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin text-primary dark:text-emerald-500" />
                    <span>Preparing...</span>
                  </>
                ) : (
                  <>
                    <Download className="h-4 w-4 text-slate-500 dark:text-slate-400" />
                    <span>Download / View</span>
                    <ExternalLink className="h-3 w-3 opacity-60" />
                  </>
                )}
              </button>

              <button
                type="button"
                onClick={() => setShowUploaderOverride(true)}
                disabled={!isVerified}
                className="inline-flex items-center gap-1.5 rounded-[10px] bg-slate-100 px-3.5 py-2 text-xs font-semibold text-slate-700 transition hover:bg-slate-200 disabled:opacity-40 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                <span>Replace Resume</span>
              </button>
            </div>
          </div>

          <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between text-[11px] text-slate-400 dark:text-slate-500">
            <span>Secure 15-minute presigned S3 URLs generate on-demand.</span>
            <span>Max 5MB • PDF or DOCX</span>
          </div>
        </div>
      ) : (
        /* State 2: Drag & Drop Uploader Area */
        <div
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={() => {
            if (isVerified && !isUploading) {
              fileInputRef.current?.click();
            }
          }}
          className={`relative flex flex-col items-center justify-center rounded-[10px] border-2 border-dashed p-8 text-center transition-all ${
            !isVerified
              ? "cursor-not-allowed border-border bg-secondary/50 opacity-70"
              : isDragging
              ? "cursor-copy border-primary bg-accent/40 scale-[1.01]"
              : "cursor-pointer border-border bg-card hover:border-primary hover:bg-accent/20"
          }`}
        >
          <div
            className={`flex h-14 w-14 items-center justify-center rounded-[10px] shadow-sm transition ${
              isDragging
                ? "bg-primary text-primary-foreground scale-110"
                : "bg-accent text-primary"
            }`}
          >
            {isUploading ? (
              <Loader2 className="h-7 w-7 animate-spin" />
            ) : (
              <UploadCloud className="h-7 w-7" />
            )}
          </div>

          <div className="mt-4 space-y-1">
            <h4 className="text-sm font-semibold text-slate-900 dark:text-white">
              {isUploading
                ? "Streaming to MinIO S3 Object Storage..."
                : isDragging
                ? "Drop your resume file here"
                : "Drag & drop your resume, or browse files"}
            </h4>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              PDF or DOCX documents up to 5MB
            </p>
          </div>

          <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
            <button
              type="button"
              disabled={!isVerified || isUploading}
              onClick={(e) => {
                e.stopPropagation();
                if (isVerified && !isUploading) {
                  fileInputRef.current?.click();
                }
              }}
              className="inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white shadow-sm shadow-sm transition hover:bg-primary disabled:opacity-50"
            >
              {isUploading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Uploading...</span>
                </>
              ) : (
                <>
                  <UploadCloud className="h-3.5 w-3.5" />
                  <span>Choose File</span>
                </>
              )}
            </button>

            {hasResume && showUploaderOverride && (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setShowUploaderOverride(false);
                  setErrorMessage(null);
                }}
                className="rounded-[10px] border border-slate-200 bg-white px-3.5 py-2 text-xs font-medium text-slate-600 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-300"
              >
                Keep Current Resume
              </button>
            )}
          </div>

          {!isVerified && (
            <p className="mt-3 text-[11px] font-medium text-amber-700 dark:text-amber-400">
              Uploader locked until university email is verified.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
