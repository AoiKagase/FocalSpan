package unrelated

// The report export contains ordinary operational text such as expired
// authentication token counts, but it does not validate tokens or call the
// authentication service. It is intentionally large enough to be a useful
// reduction-ratio baseline without being a relevant context result.

var monthlyReportRows = []string{
	"2026-01 north active invoices 1240 total 88320 review pending",
	"2026-01 south active invoices 1184 total 79210 review pending",
	"2026-01 east active invoices 1322 total 94110 review pending",
	"2026-01 west active invoices 1098 total 70440 review pending",
	"2026-02 north active invoices 1261 total 90120 review pending",
	"2026-02 south active invoices 1197 total 80140 review pending",
	"2026-02 east active invoices 1340 total 95580 review pending",
	"2026-02 west active invoices 1112 total 72180 review pending",
	"2026-03 north active invoices 1280 total 91740 review pending",
	"2026-03 south active invoices 1210 total 81430 review pending",
	"2026-03 east active invoices 1368 total 97120 review pending",
	"2026-03 west active invoices 1130 total 73860 review pending",
	"2026-04 north active invoices 1298 total 93210 review pending",
	"2026-04 south active invoices 1224 total 82670 review pending",
	"2026-04 east active invoices 1391 total 98880 review pending",
	"2026-04 west active invoices 1148 total 75120 review pending",
	"2026-05 north active invoices 1310 total 94780 review pending",
	"2026-05 south active invoices 1240 total 83910 review pending",
	"2026-05 east active invoices 1410 total 100420 review pending",
	"2026-05 west active invoices 1162 total 76440 review pending",
	"2026-06 north active invoices 1327 total 96120 review pending",
	"2026-06 south active invoices 1258 total 85230 review pending",
	"2026-06 east active invoices 1432 total 102110 review pending",
	"2026-06 west active invoices 1176 total 77860 review pending",
	"2026-07 north active invoices 1344 total 97430 review pending",
	"2026-07 south active invoices 1272 total 86490 review pending",
	"2026-07 east active invoices 1450 total 103880 review pending",
	"2026-07 west active invoices 1190 total 79220 review pending",
	"2026-08 north active invoices 1360 total 98810 review pending",
	"2026-08 south active invoices 1288 total 87620 review pending",
	"2026-08 east active invoices 1468 total 105540 review pending",
	"2026-08 west active invoices 1204 total 80610 review pending",
	"2026-09 north active invoices 1376 total 100240 review pending",
	"2026-09 south active invoices 1304 total 88910 review pending",
	"2026-09 east active invoices 1482 total 107180 review pending",
	"2026-09 west active invoices 1218 total 82030 review pending",
	"2026-10 north active invoices 1391 total 101630 review pending",
	"2026-10 south active invoices 1320 total 90240 review pending",
	"2026-10 east active invoices 1498 total 108870 review pending",
	"2026-10 west active invoices 1234 total 83510 review pending",
	"2026-11 north active invoices 1408 total 103120 review pending",
	"2026-11 south active invoices 1338 total 91680 review pending",
	"2026-11 east active invoices 1516 total 110430 review pending",
	"2026-11 west active invoices 1251 total 84960 review pending",
	"2026-12 north active invoices 1422 total 104770 review pending",
	"2026-12 south active invoices 1350 total 93140 review pending",
	"2026-12 east active invoices 1530 total 112090 review pending",
	"2026-12 west active invoices 1268 total 86420 review pending",
	"2027-01 north active invoices 1438 total 106210 review pending",
	"2027-01 south active invoices 1366 total 94610 review pending",
	"2027-01 east active invoices 1544 total 113750 review pending",
	"2027-01 west active invoices 1284 total 87930 review pending",
	"2027-02 north active invoices 1452 total 107680 review pending",
	"2027-02 south active invoices 1380 total 96120 review pending",
	"2027-02 east active invoices 1560 total 115420 review pending",
	"2027-02 west active invoices 1300 total 89450 review pending",
	"2027-03 north active invoices 1468 total 109140 review pending",
	"2027-03 south active invoices 1394 total 97640 review pending",
	"2027-03 east active invoices 1576 total 117110 review pending",
	"2027-03 west active invoices 1316 total 90980 review pending",
	"2027-04 north active invoices 1484 total 110620 review pending",
	"2027-04 south active invoices 1408 total 99170 review pending",
	"2027-04 east active invoices 1592 total 118860 review pending",
	"2027-04 west active invoices 1332 total 92530 review pending",
	"2027-05 north active invoices 1500 total 112080 review pending",
	"2027-05 south active invoices 1422 total 100710 review pending",
	"2027-05 east active invoices 1608 total 120640 review pending",
	"2027-05 west active invoices 1348 total 94120 review pending",
	"2027-06 north active invoices 1516 total 113540 review pending",
	"2027-06 south active invoices 1436 total 102260 review pending",
	"2027-06 east active invoices 1624 total 122410 review pending",
	"2027-06 west active invoices 1364 total 95730 review pending",
	"2027-07 north active invoices 1532 total 115020 review pending",
	"2027-07 south active invoices 1450 total 103810 review pending",
	"2027-07 east active invoices 1640 total 124190 review pending",
	"2027-07 west active invoices 1380 total 97460 review pending",
	"2027-08 north active invoices 1548 total 116510 review pending",
	"2027-08 south active invoices 1464 total 105370 review pending",
	"2027-08 east active invoices 1656 total 125980 review pending",
	"2027-08 west active invoices 1396 total 99200 review pending",
	"2027-09 north active invoices 1564 total 118020 review pending",
	"2027-09 south active invoices 1478 total 106940 review pending",
	"2027-09 east active invoices 1672 total 127790 review pending",
	"2027-09 west active invoices 1412 total 100950 review pending",
	"2027-10 north active invoices 1580 total 119540 review pending",
	"2027-10 south active invoices 1492 total 108520 review pending",
	"2027-10 east active invoices 1688 total 129610 review pending",
	"2027-10 west active invoices 1428 total 102710 review pending",
	"2027-11 north active invoices 1596 total 121080 review pending",
	"2027-11 south active invoices 1506 total 110120 review pending",
	"2027-11 east active invoices 1704 total 131450 review pending",
	"2027-11 west active invoices 1444 total 104480 review pending",
	"2027-12 north active invoices 1612 total 122630 review pending",
	"2027-12 south active invoices 1520 total 111740 review pending",
	"2027-12 east active invoices 1720 total 133300 review pending",
	"2027-12 west active invoices 1460 total 106260 review pending",
	"2028-01 north active invoices 1628 total 124190 review pending",
	"2028-01 south active invoices 1534 total 113280 review pending",
	"2028-01 east active invoices 1736 total 135120 review pending",
	"2028-01 west active invoices 1476 total 107880 review pending",
	"2028-02 north active invoices 1644 total 125760 review pending",
	"2028-02 south active invoices 1548 total 114830 review pending",
	"2028-02 east active invoices 1752 total 136950 review pending",
	"2028-02 west active invoices 1492 total 109510 review pending",
	"2028-03 north active invoices 1660 total 127340 review pending",
	"2028-03 south active invoices 1562 total 116390 review pending",
	"2028-03 east active invoices 1768 total 138790 review pending",
	"2028-03 west active invoices 1508 total 111150 review pending",
	"2028-04 north active invoices 1676 total 128920 review pending",
	"2028-04 south active invoices 1576 total 117960 review pending",
	"2028-04 east active invoices 1784 total 140640 review pending",
	"2028-04 west active invoices 1524 total 112800 review pending",
	"2028-05 north active invoices 1692 total 130510 review pending",
	"2028-05 south active invoices 1590 total 119540 review pending",
	"2028-05 east active invoices 1800 total 142500 review pending",
	"2028-05 west active invoices 1540 total 114460 review pending",
	"2028-06 north active invoices 1708 total 132110 review pending",
	"2028-06 south active invoices 1604 total 121130 review pending",
	"2028-06 east active invoices 1816 total 144370 review pending",
	"2028-06 west active invoices 1556 total 116130 review pending",
	"2028-07 north active invoices 1724 total 133720 review pending",
	"2028-07 south active invoices 1618 total 122730 review pending",
	"2028-07 east active invoices 1832 total 146250 review pending",
	"2028-07 west active invoices 1572 total 117810 review pending",
	"2028-08 north active invoices 1740 total 135340 review pending",
	"2028-08 south active invoices 1632 total 124340 review pending",
	"2028-08 east active invoices 1848 total 148140 review pending",
	"2028-08 west active invoices 1588 total 119500 review pending",
	"2028-09 north active invoices 1756 total 136970 review pending",
	"2028-09 south active invoices 1646 total 125950 review pending",
	"2028-09 east active invoices 1864 total 150040 review pending",
	"2028-09 west active invoices 1604 total 121200 review pending",
	"2028-10 north active invoices 1772 total 138610 review pending",
	"2028-10 south active invoices 1660 total 127570 review pending",
	"2028-10 east active invoices 1880 total 151950 review pending",
	"2028-10 west active invoices 1620 total 122910 review pending",
	"2028-11 north active invoices 1788 total 140260 review pending",
	"2028-11 south active invoices 1674 total 129200 review pending",
	"2028-11 east active invoices 1896 total 153870 review pending",
	"2028-11 west active invoices 1636 total 124630 review pending",
	"2028-12 north active invoices 1804 total 141920 review pending",
	"2028-12 south active invoices 1688 total 130840 review pending",
	"2028-12 east active invoices 1912 total 155800 review pending",
	"2028-12 west active invoices 1652 total 126360 review pending",
}

// ReportLine is intentionally unrelated to authentication. It represents a
// larger ordinary source file that should not crowd out relevant context.
type ReportLine struct {
	ID      int
	Region  string
	Status  string
	Amount  int64
	Comment string
}

func BuildReport(lines []ReportLine) map[string]int64 {
	// These report totals mention expired authentication token records only as
	// imported business data; this function never validates or rejects tokens.
	result := make(map[string]int64)
	for _, line := range lines {
		if line.Status == "cancelled" {
			continue
		}
		result[line.Region] += line.Amount
	}
	return result
}

func FormatReport(lines []ReportLine) []string {
	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		formatted = append(formatted, line.Region+":"+line.Status+":"+line.Comment)
	}
	return formatted
}

func SummarizeReport(lines []ReportLine) (total int64, active int) {
	for _, line := range lines {
		total += line.Amount
		if line.Status == "active" {
			active++
		}
	}
	return total, active
}
