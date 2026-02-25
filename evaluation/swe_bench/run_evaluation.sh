#!/bin/bash

# SWE-bench 统一评估脚本
# 支持 lite(300)、full(2294)、verified(500) 数据集

set -e

# 配置
ALEX_BIN="../../alex"
CONFIG_FILE="./config.yaml"
REAL_INSTANCES_FILE="./real_instances.json"
DEFAULT_OUTPUT="./verified_evaluation_results"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

print_header() {
    echo -e "${PURPLE}================================================${NC}"
    echo -e "${PURPLE}🏆 SWE-bench Verified 评估系统${NC}"
    echo -e "${PURPLE}================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${PURPLE}================================================${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# 显示帮助
show_help() {
    cat << EOF
🏆 SWE-bench 评估脚本

支持 SWE-bench Lite(300)、Full(2294)、Verified(500) 数据集。
默认使用真实的 SWE-bench Verified 实例进行测试。

用法: $0 [COMMAND] [OPTIONS]

评估模式:
  quick-test     快速测试（3个真实实例）- 验证系统功能
  small-batch    小批量评估（50个实例）- 初步评估  
  medium-batch   中等批量评估（150个实例）- 详细评估
  full           完整评估（500个实例）- 完整基准测试
  custom         自定义评估 - 灵活配置
  real-test      使用真实SWE-bench实例测试（推荐）
  
选项:
  -m, --model MODEL      模型名称 (默认: deepseek/deepseek-chat-v3-0324:free)
  -w, --workers NUM      Worker数量 (默认: 4)
  -o, --output DIR       输出目录 (默认: $DEFAULT_OUTPUT)
  -t, --timeout SEC      超时时间 (默认: 600秒)
  -l, --limit NUM        实例数量限制
  -s, --slice START,END  实例范围 (如: 0,100)
  --temperature TEMP     模型温度 (默认: 0.1)
  --max-tokens NUM       最大token数 (默认: 8000)
  --cost-limit COST      成本限制 (默认: 20.0)
  --shuffle              随机化实例顺序
  --resume DIR           从之前的结果恢复
  -h, --help            显示帮助

推荐评估策略:
  1. 先运行 quick-test 验证系统
  2. 再运行 small-batch 进行初步评估
  3. 根据结果决定是否运行 full 评估

高性能模型示例:
  $0 full -m "openai/gpt-4o" -w 6 --timeout 1200 --cost-limit 100
  $0 medium-batch -m "anthropic/claude-3-5-sonnet" -w 4
  
资源受限示例:
  $0 small-batch -w 2 --timeout 300 --cost-limit 10
  $0 custom -l 20 -w 1

环境变量:
  PROXY_URL          代理地址 (如: http://127.0.0.1:8118)
  OPENAI_API_KEY     OpenAI API密钥
  ANTHROPIC_API_KEY  Anthropic API密钥

EOF
}

# 检查依赖和环境
check_environment() {
    print_header "环境检查"
    
    # 检查 Alex 二进制
    if [ ! -f "$ALEX_BIN" ]; then
        print_error "找不到 Alex 二进制文件: $ALEX_BIN"
        print_info "请先运行 'make build' 构建项目"
        exit 1
    fi
    print_success "Alex 二进制文件: $ALEX_BIN"
    
    # 检查配置文件
    if [ ! -f "$CONFIG_FILE" ]; then
        print_error "找不到配置文件: $CONFIG_FILE"
        exit 1
    fi
    print_success "配置文件: $CONFIG_FILE"
    
    # 设置代理
    if [ -n "$PROXY_URL" ]; then
        export https_proxy="$PROXY_URL"
        export http_proxy="$PROXY_URL"
        print_success "代理设置: $PROXY_URL"
    fi
    
    # 检查 API 密钥
    if [ -n "$OPENAI_API_KEY" ]; then
        print_success "检测到 OpenAI API 密钥"
    fi
    
    if [ -n "$ANTHROPIC_API_KEY" ]; then
        print_success "检测到 Anthropic API 密钥"
    fi
    
    echo
}

# 创建动态配置
create_config() {
    local temp_config="./temp_verified_config.yaml"
    
    # 复制基础配置
    cp "$CONFIG_FILE" "$temp_config"
    
    # 动态更新配置
    if [ -n "$MODEL" ]; then
        sed -i '' "s/name: \".*\"/name: \"$MODEL\"/" "$temp_config"
    fi
    
    if [ -n "$WORKERS" ]; then
        sed -i '' "s/num_workers: .*/num_workers: $WORKERS/" "$temp_config"
    fi
    
    if [ -n "$OUTPUT_DIR" ]; then
        sed -i '' "s|output_path: \".*\"|output_path: \"$OUTPUT_DIR\"|" "$temp_config"
    fi
    
    if [ -n "$TIMEOUT" ]; then
        sed -i '' "s/timeout: .*/timeout: $TIMEOUT/" "$temp_config"
    fi
    
    if [ -n "$TEMPERATURE" ]; then
        sed -i '' "s/temperature: .*/temperature: $TEMPERATURE/" "$temp_config"
    fi
    
    if [ -n "$MAX_TOKENS" ]; then
        sed -i '' "s/max_tokens: .*/max_tokens: $MAX_TOKENS/" "$temp_config"
    fi
    
    if [ -n "$COST_LIMIT" ]; then
        sed -i '' "s/cost_limit: .*/cost_limit: $COST_LIMIT/" "$temp_config"
    fi
    
    echo "$temp_config"
}

# 运行评估
run_evaluation() {
    local mode="$1"
    local config_file="$2"
    
    print_header "运行 SWE-bench Verified 评估 - $mode 模式"
    
    # 构建命令
    local cmd="$ALEX_BIN run-batch"
    
    if [ -n "$config_file" ] && [ -f "$config_file" ]; then
        cmd="$cmd --config $config_file"
    else
        # 使用命令行参数
        cmd="$cmd --dataset.subset verified --dataset.split dev"
        cmd="$cmd --workers ${WORKERS:-4}"
        cmd="$cmd --output ${OUTPUT_DIR:-$DEFAULT_OUTPUT}"
        cmd="$cmd --model ${MODEL:-deepseek/deepseek-chat-v3-0324:free}"
        cmd="$cmd --timeout ${TIMEOUT:-600}"
        
        if [ -n "$INSTANCE_LIMIT" ]; then
            cmd="$cmd --instance-limit $INSTANCE_LIMIT"
        fi
        
        if [ -n "$INSTANCE_SLICE" ]; then
            cmd="$cmd --instance-slice $INSTANCE_SLICE"
        fi
        
        if [ "$SHUFFLE" = "true" ]; then
            cmd="$cmd --shuffle"
        fi
        
        if [ -n "$RESUME_DIR" ]; then
            cmd="$cmd --resume $RESUME_DIR"
        fi
    fi
    
    print_info "执行命令: $cmd"
    echo
    
    # 记录开始时间
    local start_time=$(date +%s)
    
    # 执行评估
    eval "$cmd"
    local exit_code=$?
    
    # 记录结束时间
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    if [ $exit_code -eq 0 ]; then
        print_success "评估完成！耗时: ${duration}秒"
    else
        print_error "评估失败，退出码: $exit_code"
        return $exit_code
    fi
}

# 分析结果
analyze_verified_results() {
    local result_dir="$1"
    
    print_header "SWE-bench Verified 结果分析"
    
    if [ ! -d "$result_dir" ]; then
        print_error "结果目录不存在: $result_dir"
        return 1
    fi
    
    print_info "结果目录: $result_dir"
    echo "文件列表:"
    ls -la "$result_dir/"
    echo
    
    # 分析摘要结果
    if [ -f "$result_dir/summary.json" ]; then
        print_success "📊 评估摘要报告:"
        if command -v jq >/dev/null 2>&1; then
            cat "$result_dir/summary.json" | jq '{
                "🎯 数据集": .dataset_subset,
                "📝 总任务数": .total_tasks,
                "✅ 完成任务": .completed_tasks,
                "❌ 失败任务": .failed_tasks,
                "🏆 成功率": (.success_rate | tostring + "%"),
                "⏱️ 总耗时": .duration,
                "📈 平均耗时": .avg_duration,
                "💰 总成本": ("$" + (.total_cost | tostring)),
                "🤖 使用模型": .model_name,
                "👥 Worker数": .num_workers
            }'
        else
            cat "$result_dir/summary.json"
        fi
        echo
        
        # 成功率分析
        local success_rate=$(jq -r '.success_rate' "$result_dir/summary.json" 2>/dev/null || echo "0")
        if (( $(echo "$success_rate >= 80" | bc -l) )); then
            print_success "🌟 优秀表现！成功率达到 $success_rate%"
        elif (( $(echo "$success_rate >= 60" | bc -l) )); then
            print_warning "📈 良好表现，成功率 $success_rate%，还有提升空间"
        else
            print_warning "📉 成功率 $success_rate% 偏低，建议调整模型或参数"
        fi
    fi
    
    # 分析预测结果
    if [ -f "$result_dir/preds.json" ]; then
        local pred_count=$(jq length "$result_dir/preds.json" 2>/dev/null || echo "0")
        print_success "📄 生成预测数量: $pred_count"
        
        if command -v jq >/dev/null 2>&1 && [ "$pred_count" -gt 0 ]; then
            echo
            print_info "🔍 预测质量分析:"
            
            # 状态分布
            local completed=$(jq '[.[] | select(.status == "completed")] | length' "$result_dir/preds.json")
            local failed=$(jq '[.[] | select(.status == "failed")] | length' "$result_dir/preds.json")
            
            echo "  - ✅ 成功完成: $completed"
            echo "  - ❌ 失败: $failed"
            
            # 平均耗时
            local avg_duration=$(jq '[.[] | select(.duration_seconds != null) | .duration_seconds] | add / length' "$result_dir/preds.json" 2>/dev/null || echo "N/A")
            echo "  - ⏱️ 平均耗时: ${avg_duration}秒"
            
            # 成本分析
            local total_cost=$(jq '[.[] | select(.cost != null) | .cost] | add' "$result_dir/preds.json" 2>/dev/null || echo "0")
            echo "  - 💰 总成本: $${total_cost}"
            
            echo
            print_info "📋 示例预测:"
            jq '.[0] | {
                instance_id,
                status,
                duration_seconds,
                cost,
                solution: (.solution | .[0:100] + "...")
            }' "$result_dir/preds.json" 2>/dev/null || echo "无法解析预测示例"
        fi
    fi
    
    # 错误分析
    if [ -f "$result_dir/summary.json" ]; then
        local errors=$(jq -r '.error_summary // {}' "$result_dir/summary.json")
        if [ "$errors" != "{}" ] && [ "$errors" != "null" ]; then
            echo
            print_warning "🔍 错误分析:"
            echo "$errors" | jq '.' 2>/dev/null || echo "$errors"
        fi
    fi
    
    echo
    print_success "📁 详细结果文件位于: $result_dir"
    print_info "💡 建议查看 detailed_results.json 了解每个实例的详细执行过程"
}

# 清理临时文件
cleanup() {
    if [ -f "./temp_verified_config.yaml" ]; then
        rm -f "./temp_verified_config.yaml"
    fi
}

# 设置清理陷阱
trap cleanup EXIT

# 参数解析
MODEL=""
WORKERS=""
OUTPUT_DIR=""
TIMEOUT=""
INSTANCE_LIMIT=""
INSTANCE_SLICE=""
TEMPERATURE=""
MAX_TOKENS=""
COST_LIMIT=""
SHUFFLE=""
RESUME_DIR=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--model)
            MODEL="$2"
            shift 2
            ;;
        -w|--workers)
            WORKERS="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -l|--limit)
            INSTANCE_LIMIT="$2"
            shift 2
            ;;
        -s|--slice)
            INSTANCE_SLICE="$2"
            shift 2
            ;;
        --temperature)
            TEMPERATURE="$2"
            shift 2
            ;;
        --max-tokens)
            MAX_TOKENS="$2"
            shift 2
            ;;
        --cost-limit)
            COST_LIMIT="$2"
            shift 2
            ;;
        --shuffle)
            SHUFFLE="true"
            shift
            ;;
        --resume)
            RESUME_DIR="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        quick-test|small-batch|medium-batch|full|custom|real-test)
            COMMAND="$1"
            shift
            ;;
        *)
            print_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

# 主执行逻辑
main() {
    check_environment
    
    local config_file=""
    
    case "${COMMAND:-real-test}" in
        real-test)
            print_warning "🧪 真实实例测试模式 - 评估 3 个真实 SWE-bench 实例"
            # 使用真实实例文件
            if [ -f "$REAL_INSTANCES_FILE" ]; then
                cmd="$ALEX_BIN run-batch --dataset.type file --dataset.file $(pwd)/$REAL_INSTANCES_FILE"
                cmd="$cmd --workers 1 --output ${OUTPUT_DIR:-./real_test_results}"
                print_info "使用真实 SWE-bench 实例: $REAL_INSTANCES_FILE"
                echo
                eval "$cmd"
                return $?
            else
                print_error "真实实例文件不存在: $REAL_INSTANCES_FILE"
                exit 1
            fi
            ;;
        quick-test)
            print_warning "🧪 快速测试模式 - 评估 5 个实例（网络下载）"
            INSTANCE_LIMIT="5"
            WORKERS="1"
            OUTPUT_DIR="${OUTPUT_DIR:-./verified_quick_test}"
            ;;
        small-batch)
            print_warning "📊 小批量模式 - 评估 50 个实例"
            INSTANCE_LIMIT="50"
            WORKERS="${WORKERS:-3}"
            OUTPUT_DIR="${OUTPUT_DIR:-./verified_small_batch}"
            ;;
        medium-batch)
            print_warning "📈 中等批量模式 - 评估 150 个实例"
            INSTANCE_LIMIT="150"
            WORKERS="${WORKERS:-4}"
            OUTPUT_DIR="${OUTPUT_DIR:-./verified_medium_batch}"
            ;;
        full)
            print_warning "🚀 完整评估模式 - 评估全部 500 个实例"
            print_warning "这将消耗大量时间和资源（预计 2-6 小时）"
            read -p "确认要继续完整评估吗？(y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                print_info "取消评估"
                exit 0
            fi
            WORKERS="${WORKERS:-6}"
            OUTPUT_DIR="${OUTPUT_DIR:-./verified_full_evaluation}"
            ;;
        custom)
            print_info "🔧 自定义评估模式"
            OUTPUT_DIR="${OUTPUT_DIR:-./verified_custom}"
            ;;
        *)
            print_error "未知命令: $COMMAND"
            show_help
            exit 1
            ;;
    esac
    
    # 创建配置文件（如果需要）
    if [ -n "$MODEL" ] || [ -n "$WORKERS" ] || [ -n "$OUTPUT_DIR" ] || [ -n "$TIMEOUT" ]; then
        config_file=$(create_config)
        print_info "使用动态配置文件: $config_file"
    else
        config_file="$CONFIG_FILE"
        print_info "使用默认配置文件: $config_file"
    fi
    
    # 运行评估
    run_evaluation "$COMMAND" "$config_file"
    
    # 分析结果
    local result_dir="${OUTPUT_DIR:-$DEFAULT_OUTPUT}"
    analyze_verified_results "$result_dir"
    
    print_header "🎉 SWE-bench Verified 评估完成"
    print_success "感谢使用 Alex SWE-bench Verified 评估系统！"
    print_info "如需技术支持，请查看项目文档或提交 Issue"
}

# 执行主函数
main