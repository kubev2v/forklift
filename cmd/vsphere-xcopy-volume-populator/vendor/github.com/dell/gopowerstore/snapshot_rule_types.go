/*
 *
 * Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

// SnapshotRuleIntervalEnum - Interval between snapshots taken by a snapshot rule.
type SnapshotRuleIntervalEnum string

// SnapshotRuleIntervalEnum known intervals
const (
	SnapshotRuleIntervalEnumFiveMinutes    SnapshotRuleIntervalEnum = "Five_Minutes"
	SnapshotRuleIntervalEnumFifteenMinutes SnapshotRuleIntervalEnum = "Fifteen_Minutes"
	SnapshotRuleIntervalEnumThirtyMinutes  SnapshotRuleIntervalEnum = "Thirty_Minutes"
	SnapshotRuleIntervalEnumOneHour        SnapshotRuleIntervalEnum = "One_Hour"
	SnapshotRuleIntervalEnumTwoHours       SnapshotRuleIntervalEnum = "Two_Hours"
	SnapshotRuleIntervalEnumThreeHours     SnapshotRuleIntervalEnum = "Three_Hours"
	SnapshotRuleIntervalEnumFourHours      SnapshotRuleIntervalEnum = "Four_Hours"
	SnapshotRuleIntervalEnumSixHours       SnapshotRuleIntervalEnum = "Six_Hours"
	SnapshotRuleIntervalEnumEightHours     SnapshotRuleIntervalEnum = "Eight_Hours"
	SnapshotRuleIntervalEnumTwelveHours    SnapshotRuleIntervalEnum = "Twelve_Hours"
	SnapshotRuleIntervalEnumOneDay         SnapshotRuleIntervalEnum = "One_Day"
)

// TimeZoneEnum defines identifier for timezone
type TimeZoneEnum string

// TimeZoneEnum known timezones
const (
	TimeZoneEnumEtcGMTplus12         TimeZoneEnum = "Etc__GMT_plus_12"
	TimeZoneEnumUSSamoa              TimeZoneEnum = "US__Samoa"
	TimeZoneEnumEtcGMTPlus11         TimeZoneEnum = "Etc__GMT_plus_11"
	TimeZoneEnumAmericaAtka          TimeZoneEnum = "America__Atka"
	TimeZoneEnumUSHawaii             TimeZoneEnum = "US__Hawaii"
	TimeZoneEnumEtcGMTPlus10         TimeZoneEnum = "Etc__GMT_plus_10"
	TimeZoneEnumPacificMarquesas     TimeZoneEnum = "Pacific__Marquesas"
	TimeZoneEnumUSAlaska             TimeZoneEnum = "US__Alaska"
	TimeZoneEnumPacificGambier       TimeZoneEnum = "Pacific__Gambier"
	TimeZoneEnumEtcGMTPlus9          TimeZoneEnum = "Etc__GMT_plus_9"
	TimeZoneEnumPST8PDT              TimeZoneEnum = "PST8PDT"
	TimeZoneEnumPacificPitcairn      TimeZoneEnum = "Pacific__Pitcairn"
	TimeZoneEnumUSPacific            TimeZoneEnum = "US__Pacific"
	TimeZoneEnumEtcGMTPlus8          TimeZoneEnum = "Etc__GMT_plus_8"
	TimeZoneEnumMexicoBajaSur        TimeZoneEnum = "Mexico__BajaSur"
	TimeZoneEnumAmericaBoise         TimeZoneEnum = "America__Boise"
	TimeZoneEnumAmericaPhoenix       TimeZoneEnum = "America__Phoenix"
	TimeZoneEnumMST7MDT              TimeZoneEnum = "MST7MDT"
	TimeZoneEnumEtcGMTPlus7          TimeZoneEnum = "Etc__GMT_plus_7"
	TimeZoneEnumCST6CDT              TimeZoneEnum = "CST6CDT"
	TimeZoneEnumAmericaChicago       TimeZoneEnum = "America__Chicago"
	TimeZoneEnumCanadaSaskatchewan   TimeZoneEnum = "Canada__Saskatchewan"
	TimeZoneEnumAmericaBahiaBanderas TimeZoneEnum = "America__Bahia_Banderas"
	TimeZoneEnumEtcGMTPlus6          TimeZoneEnum = "Etc__GMT_plus_6"
	TimeZoneEnumChileEasterIsland    TimeZoneEnum = "Chile__EasterIsland"
	TimeZoneEnumAmericaBogota        TimeZoneEnum = "America__Bogota"
	TimeZoneEnumAmericaNewYork       TimeZoneEnum = "America__New_York"
	TimeZoneEnumEST5EDT              TimeZoneEnum = "EST5EDT"
	TimeZoneEnumAmericaHavana        TimeZoneEnum = "America__Havana"
	TimeZoneEnumEtcGMTPlus5          TimeZoneEnum = "Etc__GMT_plus_5"
	TimeZoneEnumAmericaCaracas       TimeZoneEnum = "America__Caracas"
	TimeZoneEnumAmericaCuiaba        TimeZoneEnum = "America__Cuiaba"
	TimeZoneEnumAmericaSantoDomingo  TimeZoneEnum = "America__Santo_Domingo"
	TimeZoneEnumCanadaAtlantic       TimeZoneEnum = "Canada__Atlantic"
	TimeZoneEnumAmericaAsuncion      TimeZoneEnum = "America__Asuncion"
	TimeZoneEnumEtcGMTPlus4          TimeZoneEnum = "Etc__GMT_plus_4"
	TimeZoneEnumCanadaNewfoundland   TimeZoneEnum = "Canada__Newfoundland"
	TimeZoneEnumChileContinental     TimeZoneEnum = "Chile__Continental"
	TimeZoneEnumBrazilEast           TimeZoneEnum = "Brazil__East"
	TimeZoneEnumAmericaGodthab       TimeZoneEnum = "America__Godthab"
	TimeZoneEnumAmericaMiquelon      TimeZoneEnum = "America__Miquelon"
	TimeZoneEnumAmericaBuenosAires   TimeZoneEnum = "America__Buenos_Aires"
	TimeZoneEnumEtcMTPlus3           TimeZoneEnum = "Etc__GMT_plus_3"
	TimeZoneEnumAmericaNoronha       TimeZoneEnum = "America__Noronha"
	TimeZoneEnumEtcGMTPlus2          TimeZoneEnum = "Etc__GMT_plus_2"
	TimeZoneEnumAmericaScoresbysund  TimeZoneEnum = "America__Scoresbysund"
	TimeZoneEnumAtlanticCapeVerde    TimeZoneEnum = "Atlantic__Cape_Verde"
	TimeZoneEnumEtcGMTPlus1          TimeZoneEnum = "Etc__GMT_plus_1"
	TimeZoneEnumUTC                  TimeZoneEnum = "UTC"
	TimeZoneEnumEuropeLondon         TimeZoneEnum = "Europe__London"
	TimeZoneEnumAfricaCasablanca     TimeZoneEnum = "Africa__Casablanca"
	TimeZoneEnumAtlanticReykjavik    TimeZoneEnum = "Atlantic__Reykjavik"
	TimeZoneEnumAntarcticaTroll      TimeZoneEnum = "Antarctica__Troll"
	TimeZoneEnumEuropeParis          TimeZoneEnum = "Europe__Paris"
	TimeZoneEnumEuropeSarajevo       TimeZoneEnum = "Europe__Sarajevo"
	TimeZoneEnumEuropeBelgrade       TimeZoneEnum = "Europe__Belgrade"
	TimeZoneEnumEuropeRome           TimeZoneEnum = "Europe__Rome"
	TimeZoneEnumAfricaTunis          TimeZoneEnum = "Africa__Tunis"
	TimeZoneEnumEtcGMTMinus1         TimeZoneEnum = "Etc__GMT_minus_1"
	TimeZoneEnumAsiaGaza             TimeZoneEnum = "Asia__Gaza"
	TimeZoneEnumEuropeBucharest      TimeZoneEnum = "Europe__Bucharest"
	TimeZoneEnumEuropeHelsinki       TimeZoneEnum = "Europe__Helsinki"
	TimeZoneEnumAsiaBeirut           TimeZoneEnum = "Asia__Beirut"
	TimeZoneEnumAfricaHarare         TimeZoneEnum = "Africa__Harare"
	TimeZoneEnumAsiaDamascus         TimeZoneEnum = "Asia__Damascus"
	TimeZoneEnumAsiaAmman            TimeZoneEnum = "Asia__Amman"
	TimeZoneEnumEuropeTiraspol       TimeZoneEnum = "Europe__Tiraspol"
	TimeZoneEnumAsiaJerusalem        TimeZoneEnum = "Asia__Jerusalem"
	TimeZoneEnumEtcGMTMinus2         TimeZoneEnum = "Etc__GMT_minus_2"
	TimeZoneEnumAsiaBaghdad          TimeZoneEnum = "Asia__Baghdad"
	TimeZoneEnumAfricaAsmera         TimeZoneEnum = "Africa__Asmera"
	TimeZoneEnumEtcGMTMinus3         TimeZoneEnum = "Etc__GMT_minus_3"
	TimeZoneEnumAsiaTehran           TimeZoneEnum = "Asia__Tehran"
	TimeZoneEnumAsiaBaku             TimeZoneEnum = "Asia__Baku"
	TimeZoneEnumEtcGMTMinus4         TimeZoneEnum = "Etc__GMT_minus_4"
	TimeZoneEnumAsiaKabul            TimeZoneEnum = "Asia__Kabul"
	TimeZoneEnumAsiaKarachi          TimeZoneEnum = "Asia__Karachi"
	TimeZoneEnumEtcGMTMinus5         TimeZoneEnum = "Etc__GMT_minus_5"
	TimeZoneEnumAsiaKolkata          TimeZoneEnum = "Asia__Kolkata"
	TimeZoneEnumAsiaKatmandu         TimeZoneEnum = "Asia__Katmandu"
	TimeZoneEnumAsiaAlmaty           TimeZoneEnum = "Asia__Almaty"
	TimeZoneEnumEtcGMTMinus6         TimeZoneEnum = "Etc__GMT_minus_6"
	TimeZoneEnumAsiaRangoon          TimeZoneEnum = "Asia__Rangoon"
	TimeZoneEnumAsiaHovd             TimeZoneEnum = "Asia__Hovd"
	TimeZoneEnumAsiaBangkok          TimeZoneEnum = "Asia__Bangkok"
	TimeZoneEnumEtcGMTMinus7         TimeZoneEnum = "Etc__GMT_minus_7"
	TimeZoneEnumAsiaHongKong         TimeZoneEnum = "Asia__Hong_Kong"
	TimeZoneEnumAsiaBrunei           TimeZoneEnum = "Asia__Brunei"
	TimeZoneEnumAsiaSingapore        TimeZoneEnum = "Asia__Singapore"
	TimeZoneEnumEtcGMTMinus8         TimeZoneEnum = "Etc__GMT_minus_8"
	TimeZoneEnumAsiaPyongyang        TimeZoneEnum = "Asia__Pyongyang"
	TimeZoneEnumAustraliaEucla       TimeZoneEnum = "Australia__Eucla"
	TimeZoneEnumAsiaSeoul            TimeZoneEnum = "Asia__Seoul"
	TimeZoneEnumEtcGMTMinus9         TimeZoneEnum = "Etc__GMT_minus_9"
	TimeZoneEnumAustraliaDarwin      TimeZoneEnum = "Australia__Darwin"
	TimeZoneEnumAustraliaAdelaide    TimeZoneEnum = "Australia__Adelaide"
	TimeZoneEnumAustraliaSydney      TimeZoneEnum = "Australia__Sydney"
	TimeZoneEnumAustraliaBrisbane    TimeZoneEnum = "Australia__Brisbane"
	TimeZoneEnumAsiaMagadan          TimeZoneEnum = "Asia__Magadan"
	TimeZoneEnumEtcGMTMinus10        TimeZoneEnum = "Etc__GMT_minus_10"
	TimeZoneEnumAustraliaLordHowe    TimeZoneEnum = "Australia__Lord_Howe"
	TimeZoneEnumEtcGMTMinus11        TimeZoneEnum = "Etc__GMT_minus_11"
	TimeZoneEnumAsiaKamchatka        TimeZoneEnum = "Asia__Kamchatka"
	TimeZoneEnumPacificFiji          TimeZoneEnum = "Pacific__Fiji"
	TimeZoneEnumAntarcticaSouthPole  TimeZoneEnum = "Antarctica__South_Pole"
	TimeZoneEnumEtcGMTMinus12        TimeZoneEnum = "Etc__GMT_minus_12"
	TimeZoneEnumPacificChatham       TimeZoneEnum = "Pacific__Chatham"
	TimeZoneEnumPacificTongatapu     TimeZoneEnum = "Pacific__Tongatapu"
	TimeZoneEnumPacificApia          TimeZoneEnum = "Pacific__Apia"
	TimeZoneEnumEtcGMTMinus13        TimeZoneEnum = "Etc__GMT_minus_13"
	TimeZoneEnumPacificKiritimati    TimeZoneEnum = "Pacific__Kiritimati"
	TimeZoneEnumEtcGMTMinus14        TimeZoneEnum = "Etc__GMT_minus_14"
)

// DaysOfWeekEnum - days of week
type DaysOfWeekEnum string

// DaysOfWeekEnum - known days of week
const (
	DaysOfWeekEnumMonday    DaysOfWeekEnum = "Monday"
	DaysOfWeekEnumTuesday   DaysOfWeekEnum = "Tuesday"
	DaysOfWeekEnumWednesday DaysOfWeekEnum = "Wednesday"
	DaysOfWeekEnumThursday  DaysOfWeekEnum = "Thursday"
	DaysOfWeekEnumFriday    DaysOfWeekEnum = "Friday"
	DaysOfWeekEnumSaturday  DaysOfWeekEnum = "Saturday"
	DaysOfWeekEnumSunday    DaysOfWeekEnum = "Sunday"
)

// NASAccessTypeEnums - NAS filesystem snapshot access method
type NASAccessTypeEnum string

const (
	// NASAccessTypeEnumSnapshot - NAS filesystem snapshot access method - snapshot
	// the files within the snapshot may be access directly from the production file system in the .snapshot subdirectory of each directory.
	NASAccessTypeEnumSnapshot NASAccessTypeEnum = "Snapshot"

	// NASAccessTypeEnumProtocol - NAS filesystem snapshot access method - protocol
	// the entire file system snapshot may be shared and mounted on a client like any other file system, except that it is readonly.
	NASAccessTypeEnumProtocol NASAccessTypeEnum = "Protocol"
)

// PolicyManagedByEnum - defines entities who manage the instance
type PolicyManagedByEnum string

const (
	// PolicyManagedByEnumUser - instance is managed by the end user
	PolicyManagedByEnumUser PolicyManagedByEnum = "User"
	// PolicyManagedByEnumMetro - instance is managed by the peer system where the policy was assigned, in a Metro Cluster configuration
	PolicyManagedByEnumMetro PolicyManagedByEnum = "Metro"
	// PolicyManagedByEnumReplication - destination instance is managed by the source system in a Replication configuration
	PolicyManagedByEnumReplication PolicyManagedByEnum = "Replication"
	// PolicyManagedByEnumVMware_vSphere - instance is managed by the system through VMware vSphere/vCenter
	PolicyManagedByEnumVMwareVSphere PolicyManagedByEnum = "VMware_vSphere"
)

// SnapshotRuleCreate create snapshot rule request
type SnapshotRuleCreate struct {
	// Name of the snapshot rule
	// minLength: 1
	// maxLength: 128
	Name string `json:"name,omitempty"`

	// Interval between snapshots taken by a snapshot rule
	Interval SnapshotRuleIntervalEnum `json:"interval,omitempty"`

	// Time of the day to take a daily snapshot, with format "hh:mm" using a 24 hour clock
	// Either the interval parameter or the time_of_day parameter will be set, but not both.
	TimeOfDay string `json:"time_of_day,omitempty"`

	// Time zone identifier for applying the time zone to the time_of_day for a snapshot rule, including any DST effects if applicable
	// Applies only when a time_of_day is specified in the snapshot rule. Defaults to UTC if not specified.
	// Was added in version 2.0.0.0
	TimeZone TimeZoneEnum `json:"timezone,omitempty"`

	// Days of the week when the snapshot rule should be applied.
	// Days are determined based on the UTC time zone, unless the time_of_day and timezone properties are set.
	DaysOfWeek []DaysOfWeekEnum `json:"days_of_week,omitempty"`

	// Desired snapshot retention period in hours. The system will retain snapshots for this time period.
	// minimum: 0
	// maximum: 8760
	DesiredRetention int32 `json:"desired_retention,omitempty"`

	// NAS filesystem snapshot access method.
	// setting is ignored for volume, virtual_volume, and volume_group snapshots
	NASAccessType NASAccessTypeEnum `json:"nas_access_type,omitempty"`

	// Indicates whether this snapshot rule can be modified.
	// default: false
	IsReadOnly bool `json:"is_read_only,omitempty"`
}

// SnapshotRuleDelete body for SnapshotRuleDelete request
type SnapshotRuleDelete struct {
	// Specify whether all snapshots previously created by this snapshot rule should also be deleted when this rule is removed.
	// default false
	DeleteSnaps bool `json:"delete_snaps,omitempty"`
}

// SnapshotRule Details about a snapshot rule
type SnapshotRule struct {
	// Unique identifier of the snapshot rule
	ID string `json:"id,omitempty"`

	// Snapshot rule name.
	// This property supports case-insensitive filtering.
	Name string `json:"name,omitempty"`

	// Interval between snapshots taken by a snapshot rule.
	Interval SnapshotRuleIntervalEnum `json:"interval,omitempty"`

	// Time of the day to take a daily snapshot, with format "hh:mm" using a 24 hour clock
	// Either the interval parameter or the time_of_day parameter will be set, but not both.
	TimeOfDay string `json:"time_of_day,omitempty"`

	// Time zone identifier for applying the time zone to the time_of_day for a snapshot rule, including any DST effects if applicable
	// Applies only when a time_of_day is specified in the snapshot rule. Defaults to UTC if not specified.
	// Was added in version 2.0.0.0
	TimeZone TimeZoneEnum `json:"timezone,omitempty"`

	// Days of the week when the snapshot rule should be applied.
	// Days are determined based on the UTC time zone, unless the time_of_day and timezone properties are set.
	DaysOfWeek []DaysOfWeekEnum `json:"days_of_week,omitempty"`

	// Desired snapshot retention period in hours. The system will retain snapshots for this time period.
	// minimum: 0
	// maximum: 8760
	DesiredRetention int32 `json:"desired_retention,omitempty"`

	// Indicates whether this is a replica of a snapshot rule on a remote system
	// that is the source of a replication session replicating a storage resource to the local system.
	// defalut : false
	IsReplica bool `json:"is_replica,omitempty"`

	// NAS filesystem snapshot access method.
	// setting is ignored for volume, virtual_volume, and volume_group snapshots
	NASAccessType NASAccessTypeEnum `json:"nas_access_type,omitempty"`

	// Indicates whether this snapshot rule can be modified.
	// default: false
	IsReadOnly bool `json:"is_read_only,omitempty"`

	// entity that owns and manages this instance
	ManagedBy PolicyManagedByEnum `json:"managed_by,omitempty"`

	// 	Unique identifier of the managing entity based on the value of the managed_by property, as shown below:
	//         User - Empty
	//         Metro - Unique identifier of the remote system where the policy was assigned.
	//         Replication - Unique identifier of the source remote system.
	//         VMware_vSphere - Unique identifier of the owning VMware vSphere/vCenter.
	ManagedByID string `json:"managed_by_id,omitempty"`

	// Localized message string corresponding to interval
	IntervalL10n string `json:"interval_l10n,omitempty"`

	// Localized message string corresponding to timezone
	TimezoneL10n string `json:"timezone_l10n,omitempty"`

	// Localized message array corresponding to days_of_week
	DaysOfWeekL10n []string `json:"days_of_week_l10n,omitempty"`

	// Localized message string corresponding to nas_access_type
	NASAccessTypeL10n string `json:"nas_access_type_l10n,omitempty"`

	ManagedNyL10n string `json:"managed_by_l10n,omitempty"`

	Policies []ProtectionPolicy `json:"policies,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (s *SnapshotRule) Fields() []string {
	return []string{
		"id", "name",
		"interval", "time_of_day", "timezone", "days_of_week", "desired_retention",
		"is_replica", "nas_access_type", "is_read_only",
		"managed_by", "managed_by_id",
		"interval_l10n", "timezone_l10n", "days_of_week_l10n", "nas_access_type_l10n", "managed_by_l10n", "policies",
	}
}
