#!/bin/bash

# 工具调用问题诊断脚本
# 用于测试和验证工具调用ID匹配问题的修复

set -e

echo "🔧 工具调用问题诊断脚本"
echo "=========================="

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

export GOMODCACHE="${PROJECT_ROOT}/.cache/go/pkg/mod"
export GOCACHE="${PROJECT_ROOT}/.cache/go/build"
mkdir -p "${GOMODCACHE}" "${GOCACHE}"

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go环境未安装"
    exit 1
fi

echo "✅ Go环境检查通过"

# 编译项目
echo "🔄 编译 alex CLI..."
go build -o alex-debug ./cmd/alex

if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

echo "✅ 编译成功"

# 测试用例1：简单的工具调用
echo ""
echo "📋 测试用例1：简单文件操作工具调用"
echo "测试命令: ./alex-debug 'list files in current directory'"

./alex-debug "list files in current directory" --debug 2>&1 | tee debug-output-1.log

echo ""
echo "🔍 检查日志中的工具调用ID匹配情况..."
grep -E "(Expected tool call ID|Generated tool message|Missing responses)" debug-output-1.log || echo "未发现工具调用ID问题"

# 测试用例2：多个工具调用
echo ""
echo "📋 测试用例2：多个工具调用"
echo "测试命令: ./alex-debug 'search for go files and count them'"

./alex-debug "search for go files and count them" --debug 2>&1 | tee debug-output-2.log

echo ""
echo "🔍 检查多工具调用的ID匹配情况..."
grep -E "(Tool call.*CallID|Tool message.*ToolCallId|Missing responses)" debug-output-2.log || echo "未发现工具调用ID问题"

# 测试用例3：可能触发错误的复杂查询
echo ""
echo "📋 测试用例3：复杂查询（可能触发工具调用失败）"
echo "测试命令: ./alex-debug 'grep for main function in all go files and analyze the results'"

./alex-debug "grep for main function in all go files and analyze the results" --debug 2>&1 | tee debug-output-3.log

echo ""
echo "🔍 检查是否有工具调用失败或ID不匹配..."
grep -E "(ERROR.*Missing responses|ERROR.*CallID|fallback.*ID|Tool execution failed)" debug-output-3.log || echo "未发现严重的工具调用问题"

# 分析结果
echo ""
echo "📊 诊断结果汇总"
echo "================"

error_count=$(grep -c "ERROR.*Missing responses\|ERROR.*CallID\|fallback.*ID" debug-output-*.log 2>/dev/null || echo "0")
warning_count=$(grep -c "WARN.*CallID\|CallID mismatch" debug-output-*.log 2>/dev/null || echo "0")

echo "🔍 发现的错误数量: $error_count"
echo "⚠️  发现的警告数量: $warning_count"

if [ "$error_count" -eq 0 ] && [ "$warning_count" -eq 0 ]; then
    echo "✅ 工具调用ID匹配问题已修复！"
else
    echo "⚠️  仍存在一些工具调用问题，需要进一步排查"
    echo ""
    echo "详细错误信息："
    grep -E "ERROR.*Missing responses|ERROR.*CallID|fallback.*ID|WARN.*CallID" debug-output-*.log || true
fi

# 清理
echo ""
echo "🧹 清理临时文件..."
rm -f alex-debug debug-output-*.log

echo "🎉 诊断脚本执行完成" 
