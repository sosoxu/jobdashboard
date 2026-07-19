#!/usr/bin/env python3
"""生成测试用作业日志文件（仿照 docs/ 下的样例格式）。

为 qqqq/BGPCUP2026 项目的 survey1/survey2/survey3 生成 list 和 LOG 日志：
- list 文件：仿照 zlm-test-web1.job.5567.*.list 样例
  - NGP V1.0 ASCII 头部
  - 作业元信息（Job ListName/Project/Survey/User/Start Time）
  - ===  Start of Job Code  === 段落，含 GeoDiskIn/GeoDiskOut 模块参数清单
  - ===  End of Job Code  ===
  - -------------- Job Structure --------------
  - ***** Enter Module  AM ***** 段落
  - ==== Job resource report ====
  - **********  List Information for All Threads  **********
  - Module Run Time Information 段落
  - Job Information Table 段落
  - Job Start Date / Job End Date
  - ..........  Job Done Successful ........

- LOG 文件：仿照 zlm-test-web1.job.5568.*.log 样例
  - 原始文本输出，无 [INFO]/[WARN] 等前缀
  - "LOG File Name:" / "Job Name:" / "Project:" 头部
  - "Prepare Module GeoDiskIn" 等模块准备信息
  - "Load Module From : ..." 加载信息
  - "Warning : The ... does't exist!!!" 等警告（部分作业有错误）
  - "..........  Job Done Successful .........." 或 "Job Failed" 结尾

部分作业会模拟错误（缺少某些段落、含 ERROR 字样）以测试段落检测的健壮性。
"""
import os
import random
import datetime

ROOT = "/workspace/test/ngp_root/data"
PROJECTS = ["qqqq", "BGPCUP2026"]
SURVEYS = ["survey1", "survey2", "survey3"]
random.seed(2026)

MODULES = ["GeoDiskIn", "GeoDiskOut", "Migration", "Velocity", "Stack", "NMO"]


def ts_str(base_ts, offset_sec=0):
    dt = datetime.datetime.fromtimestamp(base_ts + offset_sec)
    return dt.strftime("%Y-%m-%d %H:%M:%S")


def gen_list_file(project, survey, job_id, base_ts, has_error=False):
    """生成 list 风格文件，仿照样例 zlm-test-web1.job.5567.*.list"""
    lines = []
    lines.append("")
    lines.append(" ZZZZZZ  L     M    M         TTTTT EEEEEE  SSSS   TTTTT        W    W    *   ")
    lines.append("      Z  L     MM  MM           T   E      S    S    T          W    W  * * * ")
    lines.append("     Z   L     M MM M ------    T   E      S         T   ------ W    W   ***  ")
    lines.append("   Z     L     M    M ------    T   EEEEEE SSSSSS    T   ------ W    W    *   ")
    lines.append("  Z      L     M    M           T   E           S    T          W WW W   ***  ")
    lines.append(" Z       L     M    M           T   E      S    S    T          WW  WW  * * * ")
    lines.append(" ZZZZZZ  LLLLL M    M           T   EEEEEE  SSSS     T          W    W    *   ")
    lines.append("")
    lines.append("       -----  NGP V1.0   -----")
    lines.append("")
    list_name = f"test-{project}-{survey}.job.{job_id}.J{int(base_ts*1000)}.list"
    lines.append(f"Job ListName : {list_name}")
    lines.append(f"Project      : {project}")
    lines.append(f"Survey       : {survey}")
    lines.append(f"Line         : ")
    lines.append(f"Database Name: ndp_check")
    lines.append(f"User         : testuser")
    lines.append(f"Sending Host : ")
    lines.append(f"Running Host : hp6c79-{random.randint(1,200):03d}")
    lines.append(f"Start   Time : {ts_str(base_ts)}")
    lines.append("")
    lines.append(" ===  Start of Job Code  === ")
    lines.append("    ")
    # 模块参数清单
    for mod in ["GeoDiskIn", "GeoDiskOut"]:
        lines.append("--------------------")
        lines.append(mod)
        lines.append("--------------------")
        lines.append("    ")
        lines.append("                 #: Filename of the input seismic data.")
        lines.append(f"        > Filename = 02-STAPPLY-FIELD,Null,{survey},{project}...")
        lines.append("                 #:  The first keyword pertains to seismic data index.")
        lines.append("        > First keyword code = Source")
        lines.append("                 #:  Selection range of the first keyword. ")
        lines.append("        > First keyword range = 18001,18002,1,0,0...")
        lines.append("                 #:  The second keyword pertains to seismic data index.")
        lines.append("        > Second keyword code = Trace")
        lines.append("                 #:  Selection range of the second keyword. ")
        lines.append("        > Second keyword range = 0,0,0...")
        lines.append("                 #: Set the flag when a gather is input. ")
        lines.append("        > Gather flag = Source")
        lines.append("                 #: Select the input seismic trace type.")
        lines.append("        > Trace type = 1 Valid trace")
        lines.append("                 #:  The same input range is used when multi-file input is set.")
        lines.append("        > Use same range for all files = Yes")
        lines.append("                 #: Select the time range of the input traces.")
        lines.append("        > Time window input = 0,0,0")
        lines.append("                 #: Select head word by non-sorting keywords,so as to select the input data.")
        lines.append("        > Select header word = Null")
        lines.append("                 #: Select header word range by non-sorting keywords,so as to select the input data.")
        lines.append("        > Select header range = 0,0,0...")
        lines.append("                 #: must be in front of one of the 1-5 keywords.")
        lines.append("        > Random select keyword = Null")
        lines.append("                 #: Random input range.")
        lines.append("        > Random select range = -9999,-9999,-9999...")
        lines.append("                 #: Option to input the data whether in multiple traces.")
        lines.append("        > Gather input = No")
        lines.append("                 #: Used to recreate index table,required by the seismic data has not been built successfully.")
        lines.append("        > Recreate index = No")
        lines.append("                 #: Only load trace header file option.")
        lines.append("        > Header only = No")
        lines.append("    ")
    lines.append(" ===  End of Job Code  === ")
    lines.append("")
    lines.append("-------------- Job Structure --------------")
    lines.append("")
    lines.append("GeoDiskIn_0: [pre-module:  ][post-module: GeoDiskOut_1]")
    lines.append("GeoDiskOut_1: [pre-module: GeoDiskIn_0 ][post-module: ]")
    lines.append("")
    lines.append("-------------------------------------------")
    lines.append("")
    # 模块 AM 阶段
    for mod in ["GeoDiskIn", "GeoDiskOut"]:
        lines.append("***** Enter Module  AM *****")
        lines.append(f" Module :      {mod}")
        lines.append(f" Version:      4.00")
        lines.append(f" Updated:      Fri Mar 23 10:34:40 CST 2018")
        lines.append(f"***** Exit Module {mod} AM *****")
        lines.append("")
        lines.append("ModAM: total memory 0 words")
        lines.append("ModAM: trace sample number =1750 ")
        lines.append("ModAM:request sample rate =4000 microseconds")
        lines.append("")
        lines.append("")
    lines.append("==== Job resource report ====")
    lines.append("Total CM: 0 words.")
    lines.append("==== End job resource report ====")
    lines.append("")
    lines.append("")
    lines.append("**********  List Information for All Threads  **********")
    lines.append("")
    lines.append("***** Module Index:  0 *****")
    lines.append("")
    lines.append("***** Module Index:  1 *****")
    lines.append("")
    lines.append("Module Run Time Information")
    lines.append("")
    lines.append("  Module Name              Run Time(AM)             Run Time(PM)")
    lines.append("   GeoDiskIn               0:00:00.492              0:00:07.701        ")
    lines.append("   GeoDiskOut              0:00:01.142              0:00:00.480        ")
    lines.append("")
    lines.append("Total Run Time(AM): 0:00:01.634")
    lines.append("Total Run Time(PM): 0:00:08.181")
    lines.append("")
    lines.append("Job Information Table")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(" | Mod Index |     Module Name      |  Loop Count  |  CPU Time(secs)  |     Run Time        |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(f" |     0     |   GeoDiskIn          |   4608       |   0.64           |   0:00:08.193       |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(f" |     1     |   GeoDiskOut         |   4608       |   0.66           |   0:00:01.622       |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(" |    Total CPU Time of All Modules(secs)     |            1.30                             |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(" |    Total CPU Time of Whole Job(secs)       |            2.30                             |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(" |    Total Run Time of All Modules           |            0:00:09.815                      |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append(" |    Total Run Time of Whole Job             |            0:00:15.000                      |")
    lines.append("  ------------------------------------------------------------------------------------------")
    lines.append("")
    lines.append(f"Job Start Date : {ts_str(base_ts)}")
    if has_error:
        lines.append(f"Job End   Date : {ts_str(base_ts, 5)}")
        lines.append("")
        lines.append("..........  Job Failed (exit code 1) ..........")
        lines.append("ERROR: GeoDiskIn module failed: input file not found")
        lines.append("Error : seismic path not found: /hw6p/data/geodata/data/" + project + "/" + survey + "/Seismics/02-STAPPLY-FIELD")
    else:
        lines.append(f"Job End   Date : {ts_str(base_ts, 16)}")
        lines.append("")
        lines.append("..........  Job Done Successful ..........")
    lines.append("")
    return lines


def gen_log_file(project, survey, job_id, base_ts, has_error=False):
    """生成 LOG 风格文件，仿照样例 zlm-test-web1.job.5568.*.log
    原始文本输出，无 [INFO]/[WARN] 等结构化前缀。"""
    lines = []
    log_name = f"test-{project}-{survey}.job.{job_id}.J{int(base_ts*1000)}.log"
    lines.append(f"LOG File Name: {log_name}")
    lines.append("")
    lines.append(f"Job Name: test-{project}-{survey}.job")
    lines.append(f"Project: {project}")
    lines.append(f"Survey: {survey}")
    lines.append(f"Line: ")
    lines.append(f"connected: {random.randint(10, 50)}")
    lines.append(f"jobFNinListDir = /hw6p/data/geodata//data/{project}/{survey}/list/{log_name}.job")
    lines.append("Prepare Module GeoDiskIn")
    lines.append("sid return = 2EE03E76ABA29FB989480987FCB97D24")
    lines.append("Prepare Module GeoDiskOut")
    lines.append("sid return = 2EE03E76ABA29FB989480987FCB97D24")
    lines.append("Load Module/pdl Order path: ")
    lines.append(" : /testdata/u3/GEOEAST/GeoEast5.1_dev_as79_x86/opt/fwiy/libso/:.:/testdata/u3/GEOEAST/GeoEast5.1_dev_as79_x86/libso/sdp/testso")
    lines.append(f"find /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskin.pdl")
    lines.append(f"Load Module From : /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskin.so")
    lines.append(f"Load Module PDL file From : /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskin.pdl")
    lines.append(f"find /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskout.pdl")
    lines.append(f"Load Module From : /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskout.so")
    lines.append(f"Load Module PDL file From : /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskout.pdl")
    lines.append("Link Modules sucessful ! ")
    lines.append("Initalize Job Complete! ")
    lines.append("Info : current module max channels = 1")
    lines.append("Info : current module max channels = 1")
    lines.append("********Module Index and Module Name *************")
    lines.append("Module Index and Module Name : 0  GeoDiskIn")
    lines.append("Module Index and Module Name : 1  GeoDiskOut")
    lines.append("**************************************************")
    lines.append("interfaceType = GEOEAST31")
    lines.append("<GeoDiskIn> %%%Enter AM section%%% .")
    lines.append("GeoDiskIn : interfaceType = GEOEAST31")
    lines.append("setCurrentModName = geodiskin")
    lines.append("m_outputChannels =1")
    lines.append("fileName = /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskin.header")
    lines.append("Warning : The /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskin.header does't exist!!!")
    lines.append("add headers num = 0")
    lines.append("del headers num = 0")
    lines.append("Input files:")
    lines.append(f"file 1 : 02-STAPPLY-FIELD {project} {survey} ")
    lines.append("multiseisread.cpp(396) CommonInfoSame : true HeaderInfoSame : true")
    lines.append("Open succeed,input file number : 1")
    lines.append(f"seismicPath ==/hw6p/data/geodata//data/{project}/{survey}/Seismics/02-STAPPLY-FIELD_2539620")
    lines.append("check heads:")
    lines.append("Index : 2 source_no trace_no  Recreate index : No")
    lines.append("SetIndex 2 key(s) success")
    lines.append("geodiskin.cpp(1591) keyrange range 18001 18002 1 0 0")
    lines.append("Header only : No")
    lines.append("trace_id range : 1 1 ")
    lines.append("Index info total traces : 4608")
    lines.append("Worker id -1")
    lines.append("after new mod am ")
    lines.append("ModAM: <GeoDiskIn> up module trace sample number: -1 SI:-1 us.")
    lines.append("ModAM: <GeoDiskIn> trace sample number=1750 SI=4000 us.")
    lines.append("<GeoDiskIn> %%%Exit from AM%%% .")
    lines.append("Module: GeoDiskIn ModType : IO")
    lines.append("max_channel_trs=1")
    lines.append("channels=1")
    lines.append("interfaceType = GEOEAST31")
    lines.append("<GeoDiskOut> %%%Enter AM section%%% .")
    lines.append("GeoDiskOut : interfaceType = GEOEAST31")
    lines.append("setCurrentModName = geodiskout")
    lines.append("m_outputChannels =1")
    lines.append("fileName = /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskout.header")
    lines.append("Warning : The /testdata/u3/GEOEAST/iEco_dev_V2.2_as79_x86/libso/batp/mod/geodiskout.header does't exist!!!")
    lines.append("add headers num = 0")
    lines.append("del headers num = 0")
    lines.append("")
    lines.append("        GeoDiskout Output info:")
    lines.append(f"single file : web-test-2source {project} {survey} ")
    lines.append(f"Delete main/ext seismic path:/hw6p/data/geodata//data/{project}/{survey}/Seismics/web-test-2source_2550800")
    lines.append("nseisapi.cpp delete success")
    lines.append("Test write succeed,output file number : 1")
    lines.append("Can copy header : true change times : 1 m_nCopyPos : 984 632")
    lines.append("nseisapi.cpp(2668) write sample interval : 4 format : 5 bytesPerSample : 4 time : 0 6996 samples : 1750 bytes : 7000")
    lines.append("nseisapi.cpp(2689) vol number : 196 header number : 128")
    lines.append(f"m_SlaveDir : /hw6p/data/geodata/   m_strPath : /data/{project}/{survey}/Seismics/web-test-2source_2551840   m_RealDataName : Header")
    lines.append(f"m_SlaveDir : /hw6p/data/geodata/   m_strPath : /data/{project}/{survey}/Seismics/web-test-2source_2551840   m_RealDataName : Volume")
    lines.append("")
    lines.append("        GeoDiskout Output End")
    lines.append("")
    lines.append(f"seismicPath ==/hw6p/data/geodata//data/{project}/{survey}/Seismics/web-test-2source_2551840")
    lines.append(f"gos_outputDataHisFileName ======/hw6p/data/geodata//data/{project}/{survey}/Seismics/web-test-2source_2551840/History")
    lines.append("after new mod am ")
    lines.append("ModAM: <GeoDiskOut> up module trace sample number: 1750 SI:4000 us.")
    lines.append("ModAM: <GeoDiskOut> trace sample number=1750 SI=4000 us.")
    lines.append("<GeoDiskOut> %%%Exit from AM%%% .")
    lines.append("Module: GeoDiskOut ModType : IO")
    lines.append("max_channel_trs=1")
    lines.append("channels=1")
    lines.append("==== Job resource report ====")
    lines.append("Total CM: 0 words.")
    lines.append("====== End job resource report ======")
    lines.append("Module Analyst Phase Complete! ")
    lines.append(f"Job Running Message Port : {random.randint(5000,9999)}")
    lines.append("reach last trace")
    lines.append("update = total_traces 4608")
    lines.append("update = azm_bin 0 0")
    lines.append("update = bin_no 5689 36600")
    lines.append("update = cable_in_line 0 0")
    lines.append("update = ccp 0 0")
    lines.append("update = ccp_bin 0 0")
    lines.append("update = ccp_line 0 0")
    lines.append("update = cmp 121 408")
    lines.append("update = cmp_line 5 27")
    lines.append("update = ffid 1 2")
    lines.append("update = first_sample_time 0 0")
    lines.append("update = first_valid_time 0 5.666687012")
    lines.append("update = gp_line 60900 65100")
    lines.append("update = gp_point 10400 39100")
    lines.append("update = gp_stake 6090001041 6510003911")
    lines.append("update = gp_sttn 2 60428")
    lines.append("update = last_sample_time 6996 6996")
    lines.append("update = last_valid_time 6984.239136 6996")
    lines.append("update = max_trace_value 0 0")
    lines.append("update = min_trace_value 0 0")
    lines.append("update = offset 33.67758942 5133.390625")
    lines.append("update = offset_bin 0 0")
    lines.append("update = offset_x_bin 0 0")
    lines.append("update = offset_y_bin 0 0")
    lines.append("update = ovt 0 0")
    lines.append("update = proj_offset -3584.99707 5025.599609")
    lines.append("update = source_no 18001 18002")
    lines.append("update = sp_line 61600 61800")
    lines.append("update = sp_point 22350 22350")
    lines.append("update = sp_stake 6160002236 6180002240")
    lines.append("update = sp_sttn 11376 14160")
    lines.append("file 1 traces : 4608 set ")
    lines.append("geodiskout.cpp(1923) files (no traces) : 0")
    lines.append("Module : GeoDiskIn loop count =4608")
    lines.append("Module : GeoDiskOut loop count =4608")
    lines.append("Module Execute Phase Complete! ")
    lines.append("Execute complete ! ")
    lines.append("")
    lines.append("... End of Sjob ...")
    lines.append("")
    if has_error:
        lines.append("..........  Job Failed ........")
        lines.append(f"ERROR: Job terminated with exit code 1")
        lines.append(f"Error : failed to write output file in module GeoDiskOut")
        lines.append(f"FATAL: disk write I/O error on /hw6p/data/geodata/data/{project}/{survey}/Seismics/")
    else:
        lines.append("..........  Job Done Successful ..........")
    lines.append("")
    return lines


def main():
    base_ts = int(datetime.datetime(2026, 7, 15, 10, 0, 0).timestamp())
    # 清空旧测试文件
    for project in PROJECTS:
        for survey in SURVEYS:
            for sub in ["list", "LOG"]:
                d = os.path.join(ROOT, project, survey, sub)
                if os.path.isdir(d):
                    for fn in os.listdir(d):
                        os.remove(os.path.join(d, fn))

    # 每个工区生成 2 个 list 文件和 2 个 LOG 文件，部分模拟错误
    for project in PROJECTS:
        for survey in SURVEYS:
            list_dir = os.path.join(ROOT, project, survey, "list")
            log_dir = os.path.join(ROOT, project, survey, "LOG")
            os.makedirs(list_dir, exist_ok=True)
            os.makedirs(log_dir, exist_ok=True)
            for idx in range(2):
                job_id = 5560 + idx
                ts = base_ts + idx * 600
                # survey2 第二个作业模拟错误（验证段落缺失场景）
                has_error = (survey == "survey2" and idx == 1)

                list_file = os.path.join(list_dir, f"test-{project}-{survey}-{idx+1}.list")
                list_lines = gen_list_file(project, survey, job_id, ts, has_error=has_error)
                with open(list_file, "w", encoding="utf-8") as f:
                    f.write("\n".join(list_lines))
                print(f"生成 list: {list_file} ({len(list_lines)} 行) error={has_error}")

                log_file = os.path.join(log_dir, f"test-{project}-{survey}-{idx+1}.log")
                log_lines = gen_log_file(project, survey, job_id, ts, has_error=has_error)
                with open(log_file, "w", encoding="utf-8") as f:
                    f.write("\n".join(log_lines))
                print(f"生成 log:  {log_file} ({len(log_lines)} 行) error={has_error}")

    print("\n=== 文件统计 ===")
    for project in PROJECTS:
        for survey in SURVEYS:
            list_dir = os.path.join(ROOT, project, survey, "list")
            log_dir = os.path.join(ROOT, project, survey, "LOG")
            for sub, d in [("list", list_dir), ("LOG", log_dir)]:
                files = sorted(os.listdir(d))
                total = sum(sum(1 for _ in open(os.path.join(d, f))) for f in files)
                print(f"{project}/{survey}/{sub}: {len(files)} 个文件, {total} 行")


if __name__ == "__main__":
    main()
